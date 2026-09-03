package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lk/zoek/backend/internal/errs"
	"github.com/lk/zoek/backend/internal/middleware"
	"github.com/lk/zoek/backend/internal/store"
)

// AuthHandler handles user authentication and profile.
type AuthHandler struct {
	Store      *store.Store
	JWTManager *middleware.JWTManager
}

func NewAuthHandler(s *store.Store, jwt *middleware.JWTManager) *AuthHandler {
	return &AuthHandler{Store: s, JWTManager: jwt}
}

// LoginRequest is the body for POST /api/v1/auth/login.
type LoginRequest struct {
	Code     string `json:"code"     binding:"required"`
	Nickname string `json:"nickname"`
}

// Login handles WeChat code login (MVP: code maps to a simulated openid).
// PRD §4.2-A: 桌主登录 → 创建牌桌
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}

	// MVP: derive openid from code (in production, call WeChat code2session)
	openID := "wx_" + req.Code

	nickname := req.Nickname
	if nickname == "" {
		nickname = "玩家" + req.Code[:min(4, len(req.Code))]
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
		"token":    token,
		"user_id":  user.ID,
		"nickname": user.Nickname,
	})
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
		"user_id":    user.ID,
		"nickname":   user.Nickname,
		"avatar_url": user.AvatarURL,
		"created_at": user.CreatedAt,
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
		"user_id":    user.ID,
		"nickname":   user.Nickname,
		"avatar_url": user.AvatarURL,
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
