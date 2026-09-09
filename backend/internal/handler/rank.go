package handler

import (
	"math"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lk/zoek/backend/internal/errs"
	"github.com/lk/zoek/backend/internal/middleware"
	"github.com/lk/zoek/backend/internal/rank"
	"github.com/lk/zoek/backend/internal/store"
)

// RankHandler 排位段位查询。
type RankHandler struct {
	Store *store.Store
}

func NewRankHandler(s *store.Store) *RankHandler {
	return &RankHandler{Store: s}
}

// GetMyRank handles GET /api/v1/rank/me — 我的段位、星级与赛季数据。
func (h *RankHandler) GetMyRank(c *gin.Context) {
	userID := middleware.GetUserID(c)
	user, err := h.Store.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, errs.ErrNotFound)
		return
	}

	info := rank.InfoFromStars(user.RankStars)
	totalGames := user.RankWins + user.RankDraws + user.RankLosses
	winRate := 0.0
	if totalGames > 0 {
		winRate = math.Round(float64(user.RankWins)/float64(totalGames)*1000) / 10
	}

	nextTier := ""
	if info.TierIndex < len(rank.Tiers) {
		nextTier = rank.Tiers[info.TierIndex].Name // Tiers[info.TierIndex] 即下一段
	}
	bestScore, _ := h.Store.RankBestScore(userID)

	c.JSON(http.StatusOK, gin.H{
		"tier":        info,
		"stars":       user.RankStars,
		"wins":        user.RankWins,
		"draws":       user.RankDraws,
		"losses":      user.RankLosses,
		"total_games": totalGames,
		"win_rate":    winRate,
		"streak":      user.RankStreak,
		"best_streak": user.RankBestStreak,
		"best_score":  bestScore,
		"points":      user.RankPoints,
		"next_tier":   nextTier,
		"nickname":    user.Nickname,
		"avatar_url":  user.AvatarURL,
	})
}
