package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lk/zoek/backend/internal/errs"
	"github.com/lk/zoek/backend/internal/middleware"
	"github.com/lk/zoek/backend/internal/store"
)

// PropHandler 席位互动道具事件：把 fx 动画同步给同桌玩家。
// 可见性（前端过滤）：kick 仅发送者与目标两人可见，其余道具全桌可见。
type PropHandler struct {
	Store *store.Store
}

func NewPropHandler(s *store.Store) *PropHandler {
	return &PropHandler{Store: s}
}

// CreateProp handles POST /api/v1/games/:game_id/props
// body: { to_player_id: int, type: "slipper|tea|kick|flower|dimsum" }
func (h *PropHandler) CreateProp(c *gin.Context) {
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
	if game.Status != "active" {
		c.JSON(http.StatusBadRequest, errs.ErrGameNotActive)
		return
	}
	player, pErr := h.Store.GetGamePlayer(gameID, userID)
	if pErr != nil {
		c.JSON(http.StatusForbidden, errs.ErrForbidden)
		return
	}

	var req struct {
		ToPlayerID int64  `json:"to_player_id"`
		Type       string `json:"type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}

	ev, err := h.Store.CreatePropEvent(gameID, player.ID, req.ToPlayerID, req.Type)
	if err != nil {
		c.JSON(http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusCreated, ev)
}

// ListProps handles GET /api/v1/games/:game_id/props?since_id=0
// 返回 id > since_id 的事件（时间正序），前端逐条回放动画。
func (h *PropHandler) ListProps(c *gin.Context) {
	gameID, err := strconv.ParseInt(c.Param("game_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}
	userID := middleware.GetUserID(c)
	if _, pErr := h.Store.GetGamePlayer(gameID, userID); pErr != nil {
		c.JSON(http.StatusForbidden, errs.ErrForbidden)
		return
	}
	sinceID, _ := strconv.ParseInt(c.DefaultQuery("since_id", "0"), 10, 64)
	if sinceID < 0 {
		sinceID = 0
	}
	evs, err := h.Store.GetPropEventsSince(gameID, sinceID, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}
	c.JSON(http.StatusOK, gin.H{"props": evs})
}
