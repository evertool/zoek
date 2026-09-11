package handler

import (
	"strconv"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lk/zoek/backend/internal/errs"
	"github.com/lk/zoek/backend/internal/middleware"
	"github.com/lk/zoek/backend/internal/store"
)

// LeaderboardHandler handles 雀友榜 and personal stats (PRD §1.4 P1).
type LeaderboardHandler struct {
	Store *store.Store
}

func NewLeaderboardHandler(s *store.Store) *LeaderboardHandler {
	return &LeaderboardHandler{Store: s}
}

// leaderboardWindowDays 统计最近 N 天（PRD P1：默认统计最近 30 天）
const leaderboardWindowDays = 30

// minQualifiedGames 进入正式榜单的最少完成局数（无门槛：打完 1 局即上榜）
const minQualifiedGames = 1

// maxTrendPoints 个人折线图最多展示最近 N 场
const maxTrendPoints = 20

// GetLeaderboard handles GET /api/v1/leaderboard?days=7|30|0
func (h *LeaderboardHandler) GetLeaderboard(c *gin.Context) {
	userID := middleware.GetUserID(c)
	days, _ := strconv.Atoi(c.DefaultQuery("days", strconv.Itoa(leaderboardWindowDays)))
	if days < 0 {
		days = leaderboardWindowDays
	}
	entries, err := h.Store.GetLeaderboard(userID, days, minQualifiedGames)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"days":        days,
		"min_games":   minQualifiedGames,
		"leaderboard": entries,
	})
}

// GetUserStats handles GET /api/v1/user/stats.
func (h *LeaderboardHandler) GetUserStats(c *gin.Context) {
	userID := middleware.GetUserID(c)
	stats, err := h.Store.GetUserStats(userID, maxTrendPoints)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}
	c.JSON(http.StatusOK, stats)
}

// GetUserBadges handles GET /api/v1/user/badges（PRD §3.6.6 v1.3）。
func (h *LeaderboardHandler) GetUserBadges(c *gin.Context) {
	userID := middleware.GetUserID(c)
	badges, err := h.Store.GetUserBadges(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}
	c.JSON(http.StatusOK, badges)
}
