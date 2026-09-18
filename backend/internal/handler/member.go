package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lk/zoek/backend/internal/errs"
	"github.com/lk/zoek/backend/internal/middleware"
	"github.com/lk/zoek/backend/internal/model"
)

// 房间页长按座位触发的两个动作：
//   - 长按自己  → LeaveGame  自己退出牌台
//   - 台主长按他人 → KickPlayer 把该雀友移出牌台
//
// 两者共用同一套「离座」流程（store.LeaveGamePlayer，软删除）：
//   1. 取消他身上所有「待确认」的转分（离座后没人能替他确认）
//   2. 离座（status=left，座位归零，释放给后面的人）
//   3. 台主退出时把台主转给剩下最早加入的在座玩家
//   4. 最后一个人走了 → 自动散台，不留僵尸台

type KickPlayerRequest struct {
	TargetSeat     int   `json:"target_seat"`
	TargetPlayerID int64 `json:"target_player_id"`
}

// ensureNoLedger 离座前置校验：本局已经产生流水账单就不能走退出/移出，
// 只能由台主「结束散台」统一结算——否则人走了账单挂在半空，没人对得上账。
// 返回 false 表示已经写好响应，调用方直接 return。
func (h *GameHandler) ensureNoLedger(c *gin.Context, gameID int64) bool {
	count, err := h.Store.CountAdjustments(gameID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return false
	}
	if count > 0 {
		c.JSON(http.StatusBadRequest, errs.ErrGameHasLedger)
		return false
	}
	return true
}

// LeaveGame handles POST /api/v1/games/:game_id/leave
func (h *GameHandler) LeaveGame(c *gin.Context) {
	gameID, err := strconv.ParseInt(c.Param("game_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}
	userID := middleware.GetUserID(c)

	game, err := h.Store.GetGame(gameID)
	if err != nil {
		// 台子已经没了，对用户来说等价于「已经不在台上」
		c.JSON(http.StatusBadRequest, errs.ErrGameDissolved)
		return
	}
	if game.Status != "forming" && game.Status != "active" {
		c.JSON(http.StatusBadRequest, errs.ErrGameNotJoinable)
		return
	}

	me, err := h.Store.GetGamePlayer(gameID, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrNotInGame)
		return
	}

	// 有流水账单就不给走，必须走「结束散台」
	if !h.ensureNoLedger(c, gameID) {
		return
	}

	// 台主退出前先交棒，避免剩下的人「没人能结束散台」
	newOwnerName := ""
	if game.CreatorID == userID {
		active, _ := h.Store.GetActiveGamePlayers(gameID)
		for i := range active {
			if active[i].ID == me.ID {
				continue
			}
			if err := h.Store.TransferGameOwner(gameID, me.ID, active[i].ID, active[i].UserID); err != nil {
				c.JSON(http.StatusInternalServerError, errs.ErrInternal)
				return
			}
			newOwnerName = active[i].NicknameSnapshot
			break
		}
	}

	if err := h.Store.LeaveGamePlayer(gameID, me.ID); err != nil {
		if be, ok := err.(*errs.BizError); ok {
			c.JSON(http.StatusBadRequest, be)
			return
		}
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}

	remaining, _ := h.Store.CountGamePlayers(gameID)
	message := "已退出牌台"
	switch {
	case remaining == 0:
		// 最后一个人走了：这台已经没人了。没有流水（上面 ensureNoLedger 已保证）
		// → 直接物理删除，不留「僵尸台」；真有流水（并发窗口塞进来的）才退回置 cancelled。
		deleted, dErr := h.Store.DeleteGameIfNoLedger(gameID)
		if dErr != nil {
			c.JSON(http.StatusInternalServerError, errs.ErrInternal)
			return
		}
		if deleted {
			message = "最后一位已退出，牌台已散"
			break
		}
		// 不带 expectedStatus 限制，避免状态刚变过就失败
		if _, sErr := h.Store.UpdateGameStatus(gameID, game.Status, "cancelled"); sErr == nil {
			message = "最后一位已退出，牌台已散"
		}
	case newOwnerName != "":
		message = "已退出牌台，台主转给" + newOwnerName
	}

	c.JSON(http.StatusOK, gin.H{
		"game_id":   gameID,
		"message":   message,
		"remaining": remaining,
	})
}

// KickPlayer handles POST /api/v1/games/:game_id/kick （台主把雀友移出牌台）
func (h *GameHandler) KickPlayer(c *gin.Context) {
	gameID, err := strconv.ParseInt(c.Param("game_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}
	userID := middleware.GetUserID(c)

	game, err := h.Store.GetGame(gameID)
	if err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrGameDissolved)
		return
	}
	if game.Status != "forming" && game.Status != "active" {
		c.JSON(http.StatusBadRequest, errs.ErrGameNotJoinable)
		return
	}
	// 权限：后端一律以 games.creator_id 判定台主
	if game.CreatorID != userID {
		c.JSON(http.StatusForbidden, errs.ErrOwnerOnly)
		return
	}

	var req KickPlayerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}
	if req.TargetPlayerID <= 0 && (req.TargetSeat < 1 || req.TargetSeat > 4) {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}

	// 有流水账单就不给踢，必须走「结束散台」
	if !h.ensureNoLedger(c, gameID) {
		return
	}

	players, _ := h.Store.GetActiveGamePlayers(gameID)
	var target *model.GamePlayer
	for i := range players {
		if (req.TargetPlayerID > 0 && players[i].ID == req.TargetPlayerID) ||
			(req.TargetSeat > 0 && players[i].Seat == req.TargetSeat) {
			target = &players[i]
			break
		}
	}
	if target == nil {
		c.JSON(http.StatusBadRequest, errs.ErrTargetNotSeated)
		return
	}
	if target.UserID == userID {
		c.JSON(http.StatusBadRequest, errs.ErrKickSelf)
		return
	}

	if err := h.Store.LeaveGamePlayer(gameID, target.ID); err != nil {
		if be, ok := err.(*errs.BizError); ok {
			c.JSON(http.StatusBadRequest, be)
			return
		}
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"game_id":   gameID,
		"player_id": target.ID,
		"seat":      target.Seat,
		"message":   "已请" + target.NicknameSnapshot + "离开牌台",
	})
}
