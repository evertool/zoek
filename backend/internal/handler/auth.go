package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lk/zoek/backend/internal/errs"
	"github.com/lk/zoek/backend/internal/middleware"
	"github.com/lk/zoek/backend/internal/model"
	"github.com/lk/zoek/backend/internal/rank"
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
// 优先读 User.ProfileCompleted 字段，兜底再做运行时推导。
func profileIncomplete(u *model.User) bool {
	if u.ProfileCompleted {
		return false
	}
	return u.Nickname == "" || u.AvatarURL == ""
}

// UploadAvatar handles POST /api/v1/user/avatar.
// 接收 multipart 文件，保存到 uploads/avatars/ 目录，返回相对路径。
func (h *AuthHandler) UploadAvatar(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, errs.New("NO_FILE", "请上传头像文件", errs.ActionRetry))
		return
	}

	// 限制文件大小 5MB（前端也做了压缩，这里是后端兜底）
	if file.Size > 5*1024*1024 {
		c.JSON(http.StatusBadRequest, errs.New("FILE_TOO_LARGE", "头像文件不能超过5MB", errs.ActionRetry))
		return
	}

	// 获取文件扩展名
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext == "" {
		ext = ".jpg"
	}
	// 只允许常见图片格式
	allowedExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}
	if !allowedExts[ext] {
		c.JSON(http.StatusBadRequest, errs.New("INVALID_FORMAT", "仅支持 jpg/png/webp 格式", errs.ActionRetry))
		return
	}

	userID := middleware.GetUserID(c)

	// 生成文件名：{userID}_{timestamp}{ext}
	filename := fmt.Sprintf("%d_%d%s", userID, fileHeaderToTimestamp(file.Filename), ext)
	uploadDir := "uploads/avatars"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}
	savePath := filepath.Join(uploadDir, filename)
	if err := c.SaveUploadedFile(file, savePath); err != nil {
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}

	// 返回相对路径
	relPath := "/" + filepath.ToSlash(savePath)
	c.JSON(http.StatusOK, gin.H{
		"avatar_url": relPath,
	})
}

// fileHeaderToTimestamp generates a timestamp-based suffix from filename for uniqueness.
func fileHeaderToTimestamp(_ string) int64 {
	return time.Now().Unix()
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
		"rank":         rank.InfoFromStars(user.RankStars),
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
