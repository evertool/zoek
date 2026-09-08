package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lk/zoek/backend/internal/errs"
	"github.com/lk/zoek/backend/internal/middleware"
	"github.com/lk/zoek/backend/internal/model"
	"github.com/lk/zoek/backend/internal/store"
	"github.com/lk/zoek/backend/pkg/wechat"
)

// AuthHandler handles user authentication and profile.
type AuthHandler struct {
	Store      *store.Store
	JWTManager *middleware.JWTManager
	WxClient   *wechat.Client
}

func NewAuthHandler(s *store.Store, jwt *middleware.JWTManager, wx *wechat.Client) *AuthHandler {
	return &AuthHandler{Store: s, JWTManager: jwt, WxClient: wx}
}

// LoginRequest is the body for POST /api/v1/auth/login.
type LoginRequest struct {
	Code     string `json:"code"     binding:"required"`
	Nickname string `json:"nickname"`
}

// Login handles WeChat code login.
// PRD §4.2-A: 桌主登录 → 创建牌桌
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}

	// Call WeChat code2session to get openid
	openID, _, err := h.WxClient.Code2Session(req.Code)
	if err != nil {
		c.JSON(http.StatusUnauthorized, errs.New("WX_LOGIN_FAILED", "微信登录失败", errs.ActionRetry))
		return
	}

	nickname := req.Nickname
	if nickname == "" {
		nickname = "玩家"
	}

	user, err := h.Store.FindOrCreateUser(openID, nickname, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}

	token, err := h.JWTManager.GenerateToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":        token,
		"user_id":      user.ID,
		"nickname":     user.Nickname,
		"avatar_url":   user.AvatarURL,
		"need_profile": profileIncomplete(user),
	})
}

// profileIncomplete reports whether the user still lacks nickname or avatar.
// PRD §4.2-A: 资料完整的老用户重新登录不得再次弹出完善资料页。
// chooseAvatar 的微信临时路径（http://tmp/、wxfile://）重启后失效，视同未设置。
func profileIncomplete(u *model.User) bool {
	return u.Nickname == "" || !isPersistentAvatar(u.AvatarURL)
}

// isPersistentAvatar reports whether the avatar URL survives app restarts.
// 仅接受 base64 数据 URL 和 https 地址；微信临时路径不算完整资料。
func isPersistentAvatar(url string) bool {
	return strings.HasPrefix(url, "data:image") || strings.HasPrefix(url, "https://")
}

// GetProfile handles GET /api/v1/user/profile.
func (h *AuthHandler) GetProfile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	user, err := h.Store.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, errs.ErrNotFound)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"user_id":      user.ID,
		"nickname":     user.Nickname,
		"avatar_url":   user.AvatarURL,
		"need_profile": profileIncomplete(user),
		"created_at":   user.CreatedAt,
	})
}

// UpdateProfileRequest is the body for PUT /api/v1/user/profile.
type UpdateProfileRequest struct {
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
}

// UpdateProfile handles PUT /api/v1/user/profile.
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}
	userID := middleware.GetUserID(c)
	user, err := h.Store.UpdateUserProfile(userID, req.Nickname, req.AvatarURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"user_id":      user.ID,
		"nickname":     user.Nickname,
		"avatar_url":   user.AvatarURL,
		"need_profile": profileIncomplete(user),
	})
}
