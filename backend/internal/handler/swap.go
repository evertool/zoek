package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lk/zoek/backend/internal/errs"
	"github.com/lk/zoek/backend/internal/middleware"
	"github.com/lk/zoek/backend/internal/model"
	"github.com/lk/zoek/backend/internal/store"
)

// SwapHandler handles seat swap requests between two seated players (PRD §8.7).
// 长按空位是即时换座（gameHandler.SwapSeat），长按他人座位走这里的申请/确认流程。
type SwapHandler struct {
	Store *store.Store
}

func NewSwapHandler(s *store.Store) *SwapHandler { return &SwapHandler{Store: s} }

type CreateSwapRequestRequest struct {
	TargetSeat int64 `json:"target_seat"`
}

type SwapRequestResponse struct {
	ID           int64     `json:"id"`
	GameID       int64     `json:"game_id"`
	FromPlayerID int64     `json:"from_player_id"`
	ToPlayerID   int64     `json:"to_player_id"`
	FromSeat     int       `json:"from_seat"`
	ToSeat       int       `json:"to_seat"`
	Status       string    `json:"status"`
	FromNickname string    `json:"from_nickname,omitempty"`
	ToNickname   string    `json:"to_nickname,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
}

// CreateSwapRequest handles POST /api/v1/games/:game_id/swap_requests
func (h *SwapHandler) CreateSwapRequest(c *gin.Context) {
	gameID, err := strconv.ParseInt(c.Param("game_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}
	userID := middleware.GetUserID(c)

	game, err := h.Store.GetGame(gameID)
	if err != nil {
		c.JSON(http.StatusNotFound, errs.ErrNotFound)
		return
	}
	if game.Status != "forming" && game.Status != "active" {
		c.JSON(http.StatusBadRequest, errs.ErrGameNotActive)
		return
	}

	fromPlayer, pErr := h.Store.GetGamePlayer(gameID, userID)
	if pErr != nil {
		c.JSON(http.StatusForbidden, errs.ErrForbidden)
		return
	}

	var req CreateSwapRequestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}
	if req.TargetSeat < 1 || req.TargetSeat > 4 {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}
	if int(req.TargetSeat) == fromPlayer.Seat {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}

	players, _ := h.Store.GetGamePlayers(gameID)
	var toPlayer *model.GamePlayer
	for i := range players {
		if players[i].Seat == int(req.TargetSeat) {
			toPlayer = &players[i]
			break
		}
	}
	if toPlayer == nil {
		c.JSON(http.StatusBadRequest, errs.ErrSeatEmpty)
		return
	}

	if pending, _ := h.Store.HasPendingSwapRequestFrom(gameID, fromPlayer.ID); pending {
		c.JSON(http.StatusBadRequest, errs.ErrSwapPending)
		return
	}

	swapReq := &model.SeatSwapRequest{
		GameID:       gameID,
		FromPlayerID: fromPlayer.ID,
		ToPlayerID:   toPlayer.ID,
		FromSeat:     fromPlayer.Seat,
		ToSeat:       toPlayer.Seat,
		Status:       "pending",
		ExpiresAt:    time.Now().Add(2 * time.Minute),
	}
	if err := h.Store.CreateSwapRequest(swapReq); err != nil {
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"request": SwapRequestResponse{
			ID:           swapReq.ID,
			GameID:       swapReq.GameID,
			FromPlayerID: swapReq.FromPlayerID,
			ToPlayerID:   swapReq.ToPlayerID,
			FromSeat:     swapReq.FromSeat,
			ToSeat:       swapReq.ToSeat,
			Status:       swapReq.Status,
			FromNickname: fromPlayer.NicknameSnapshot,
			ExpiresAt:    swapReq.ExpiresAt,
			CreatedAt:    swapReq.CreatedAt,
		},
		"message": fmt.Sprintf("换位申请已发送，等待%s确认", toPlayer.NicknameSnapshot),
	})
}

// GetPendingSwapRequest handles GET /api/v1/games/:game_id/swap_requests/pending
// 供房间页轮询：
//   - request  = 发给我的待处理申请（没有则 null）
//   - outgoing = 我最近发出的那条申请及其状态（用于把"对方已拒绝/已同意"回传给发起人）
func (h *SwapHandler) GetPendingSwapRequest(c *gin.Context) {
	gameID, err := strconv.ParseInt(c.Param("game_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}
	userID := middleware.GetUserID(c)

	players, _ := h.Store.GetGamePlayers(gameID)
	nicknameOf := func(playerID int64) string {
		for _, p := range players {
			if p.ID == playerID {
				return p.NicknameSnapshot
			}
		}
		return ""
	}

	var incoming *SwapRequestResponse
	req, err := h.Store.GetPendingSwapRequestForUser(gameID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}
	if req != nil {
		incoming = &SwapRequestResponse{
			ID:           req.ID,
			GameID:       req.GameID,
			FromPlayerID: req.FromPlayerID,
			ToPlayerID:   req.ToPlayerID,
			FromSeat:     req.FromSeat,
			ToSeat:       req.ToSeat,
			Status:       req.Status,
			FromNickname: nicknameOf(req.FromPlayerID),
			ExpiresAt:    req.ExpiresAt,
			CreatedAt:    req.CreatedAt,
		}
	}

	var outgoing *SwapRequestResponse
	me, pErr := h.Store.GetGamePlayer(gameID, userID)
	if pErr == nil {
		if sent, sErr := h.Store.GetLatestSwapRequestFrom(gameID, me.ID); sErr == nil && sent != nil {
			outgoing = &SwapRequestResponse{
				ID:           sent.ID,
				GameID:       sent.GameID,
				FromPlayerID: sent.FromPlayerID,
				ToPlayerID:   sent.ToPlayerID,
				FromSeat:     sent.FromSeat,
				ToSeat:       sent.ToSeat,
				Status:       sent.Status,
				ToNickname:   nicknameOf(sent.ToPlayerID),
				ExpiresAt:    sent.ExpiresAt,
				CreatedAt:    sent.CreatedAt,
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"request":  incoming,
		"outgoing": outgoing,
	})
}

// ResolveSwapRequest handles POST /api/v1/games/:game_id/swap_requests/:id/accept|reject|cancel
func (h *SwapHandler) ResolveSwapRequest(c *gin.Context) {
	gameID, err := strconv.ParseInt(c.Param("game_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}
	reqID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}
	action := c.Param("action") // accept / reject / cancel
	newStatus := map[string]string{"accept": "accepted", "reject": "rejected", "cancel": "cancelled"}[action]
	if newStatus == "" {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}
	userID := middleware.GetUserID(c)

	req, err := h.Store.GetSwapRequest(reqID)
	if err != nil {
		c.JSON(http.StatusNotFound, errs.ErrNotFound)
		return
	}
	if req.GameID != gameID {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}

	me, pErr := h.Store.GetGamePlayer(gameID, userID)
	if pErr != nil {
		c.JSON(http.StatusForbidden, errs.ErrForbidden)
		return
	}
	// 接受/拒绝：只有接收方（目标座位玩家）可以；取消：只有发起人可以
	if action == "cancel" {
		if me.ID != req.FromPlayerID {
			c.JSON(http.StatusForbidden, errs.ErrForbidden)
			return
		}
	} else if me.ID != req.ToPlayerID {
		c.JSON(http.StatusForbidden, errs.ErrForbidden)
		return
	}

	resolved, err := h.Store.ResolveSwapRequest(reqID, "pending", newStatus)
	if err != nil {
		if be, ok := err.(*errs.BizError); ok {
			c.JSON(http.StatusBadRequest, be)
			return
		}
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}

	message := "已拒绝换位申请"
	if newStatus == "accepted" {
		if err := h.Store.SwapSeats(gameID, resolved.FromPlayerID, resolved.ToPlayerID); err != nil {
			c.JSON(http.StatusInternalServerError, errs.ErrInternal)
			return
		}
		message = "已互换座位"
	} else if newStatus == "cancelled" {
		message = "已取消换位申请"
	}

	c.JSON(http.StatusOK, gin.H{
		"request_id": resolved.ID,
		"status":     resolved.Status,
		"message":    message,
	})
}
