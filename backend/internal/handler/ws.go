package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lk/zoek/backend/internal/errs"
	"github.com/lk/zoek/backend/internal/middleware"
	"github.com/lk/zoek/backend/internal/store"
	"github.com/lk/zoek/backend/internal/ws"
)

// WSHandler 房间长连接端点。
// 鉴权走 query 参数（wx.connectSocket 无法自定义请求头）：/ws?token=<jwt>&game_id=<id>
// 连接建立后服务端推送 room 事件；客户端消息仅用于保活。
type WSHandler struct {
	Store *store.Store
	JWT   *middleware.JWTManager
	Hub   *ws.Hub
}

func NewWSHandler(s *store.Store, jwt *middleware.JWTManager, hub *ws.Hub) *WSHandler {
	return &WSHandler{Store: s, JWT: jwt, Hub: hub}
}

// ServeWS handles GET /api/v1/ws.
func (h *WSHandler) ServeWS(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusUnauthorized, errs.ErrAuthExpired)
		return
	}
	claims, err := h.JWT.ParseToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, errs.ErrAuthExpired)
		return
	}
	gameID, err := strconv.ParseInt(c.Query("game_id"), 10, 64)
	if err != nil || gameID <= 0 {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}
	// 必须是该桌玩家才能挂上房间推送
	if _, pErr := h.Store.GetGamePlayer(gameID, claims.UserID); pErr != nil {
		c.JSON(http.StatusForbidden, errs.ErrForbidden)
		return
	}
	h.Hub.Serve(c.Writer, c.Request, gameID)
}
