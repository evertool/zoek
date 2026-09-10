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

// AdjustmentHandler handles score adjustment operations (PRD §3.2).
type AdjustmentHandler struct {
	Store *store.Store
}

func NewAdjustmentHandler(s *store.Store) *AdjustmentHandler {
	return &AdjustmentHandler{Store: s}
}

// ---------------------------------------------------------------------------
// Request/Response types
// ---------------------------------------------------------------------------

type CreateAdjustmentRequest struct {
	RoundID        int64  `json:"round_id"`
	ToPlayerID     int64  `json:"to_player_id"`
	AdjustmentType string `json:"adjustment_type"`
	Amount         int    `json:"amount"`
	Reason         string `json:"reason"`
	RequestID      string `json:"request_id"`
	// AutoAccept: 台间记分（比分）场景无需对方确认，建单即生效
	AutoAccept bool `json:"auto_accept"`
}

type AdjustmentResponse struct {
	ID             int64     `json:"id"`
	GameID         int64     `json:"game_id"`
	RoundID        int64     `json:"round_id"`
	FromPlayerID   int64     `json:"from_player_id"`
	ToPlayerID     int64     `json:"to_player_id"`
	AdjustmentType string    `json:"adjustment_type"`
	Amount         int       `json:"amount"`
	Reason         string    `json:"reason,omitempty"`
	Status         string    `json:"status"`
	ExpiresAt      time.Time `json:"expires_at"`
	CreatedAt      time.Time `json:"created_at"`
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// CreateAdjustment handles POST /api/v1/games/:game_id/rounds/:round_id/adjustments
// PRD §3.2: 补分/退分
func (h *AdjustmentHandler) CreateAdjustment(c *gin.Context) {
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

	// Game must be active or ended (PRD §3.2 rule 10: 24h window after end)
	if game.Status != "active" && game.Status != "ended" {
		c.JSON(http.StatusBadRequest, errs.ErrGameNotActive)
		return
	}

	// Check 24h window if ended
	if game.Status == "ended" && game.EndedAt != nil {
		if time.Now().After(game.EndedAt.Add(24 * time.Hour)) {
			c.JSON(http.StatusForbidden, errs.ErrGameEnded)
			return
		}
	}

	// Must be a player
	fromPlayer, pErr := h.Store.GetGamePlayer(gameID, userID)
	if pErr != nil {
		c.JSON(http.StatusForbidden, errs.ErrForbidden)
		return
	}

	var req CreateAdjustmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}

	// Get round_id from URL param (not body)
	req.RoundID, err = strconv.ParseInt(c.Param("round_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}

	// Validate adjustment type
	if req.AdjustmentType != "supplement" && req.AdjustmentType != "refund" {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}

	// Amount must be positive (PRD §3.2 rule 3)
	if req.Amount <= 0 {
		c.JSON(http.StatusBadRequest, errs.New("INVALID_AMOUNT", "调整数值必须为正整数", errs.ActionRetry))
		return
	}

	// To player must be different and in same game (PRD §3.2 rules 1-2)
	if req.ToPlayerID == fromPlayer.ID {
		c.JSON(http.StatusBadRequest, errs.New("SAME_PLAYER", "不能向自己发起调整", errs.ActionRetry))
		return
	}

	players, _ := h.Store.GetGamePlayers(gameID)
	toPlayerExists := false
	for _, p := range players {
		if p.ID == req.ToPlayerID {
			toPlayerExists = true
			break
		}
	}
	if !toPlayerExists {
		c.JSON(http.StatusBadRequest, errs.New("PLAYER_NOT_FOUND", "目标玩家不在本桌", errs.ActionRetry))
		return
	}

	// Verify round exists and belongs to game
	round, err := h.Store.GetRound(req.RoundID)
	if err != nil {
		c.JSON(http.StatusBadRequest, errs.New("ROUND_NOT_FOUND", "关联局不存在", errs.ActionRetry))
		return
	}
	if round.GameID != gameID {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}

	requestID := req.RequestID
	if requestID == "" {
		requestID = fmt.Sprintf("adj-%d-%d-%d", gameID, req.RoundID, time.Now().UnixNano())
	}

	now := time.Now()
	adj := &model.ScoreAdjustment{
		GameID:         gameID,
		RoundID:        req.RoundID,
		FromPlayerID:   fromPlayer.ID,
		ToPlayerID:     req.ToPlayerID,
		AdjustmentType: req.AdjustmentType,
		Amount:         req.Amount,
		Reason:         req.Reason,
		ProposedBy:     fromPlayer.ID,
		Status:         "pending",
		RequestID:      requestID,
		ExpiresAt:      now.Add(24 * time.Hour),
	}
	// 台间记分：无需对方确认，直接生效
	if req.AutoAccept {
		adj.Status = "accepted"
		resolvedAt := now
		adj.ResolvedAt = &resolvedAt
		adj.ResolvedBy = &userID
	}

	if err := h.Store.CreateAdjustment(adj); err != nil {
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}

	// Find target player nickname for message
	var toNickname string
	for _, p := range players {
		if p.ID == req.ToPlayerID {
			toNickname = p.NicknameSnapshot
			break
		}
	}

	var message string
	if req.AutoAccept {
		message = fmt.Sprintf("已转记 %d 分给 %s", req.Amount, toNickname)
	} else if req.AdjustmentType == "supplement" {
		message = fmt.Sprintf("补分请求已发送，等待%s确认", toNickname)
	} else {
		message = fmt.Sprintf("退分请求已发送，等待%s确认", toNickname)
	}

	c.JSON(http.StatusCreated, gin.H{
		"adjustment": AdjustmentResponse{
			ID:             adj.ID,
			GameID:         adj.GameID,
			RoundID:        adj.RoundID,
			FromPlayerID:   adj.FromPlayerID,
			ToPlayerID:     adj.ToPlayerID,
			AdjustmentType: adj.AdjustmentType,
			Amount:         adj.Amount,
			Reason:         adj.Reason,
			Status:         adj.Status,
			ExpiresAt:      adj.ExpiresAt,
			CreatedAt:      adj.CreatedAt,
		},
		"message": message,
	})
}

// ListAdjustments handles GET /api/v1/games/:game_id/adjustments
func (h *AdjustmentHandler) ListAdjustments(c *gin.Context) {
	gameID, err := strconv.ParseInt(c.Param("game_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}
	userID := middleware.GetUserID(c)

	_, pErr := h.Store.GetGamePlayer(gameID, userID)
	if pErr != nil {
		c.JSON(http.StatusForbidden, errs.ErrForbidden)
		return
	}

	adjs, err := h.Store.GetAdjustmentsByGameID(gameID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}

	result := make([]AdjustmentResponse, 0, len(adjs))
	for _, a := range adjs {
		result = append(result, AdjustmentResponse{
			ID:             a.ID,
			GameID:         a.GameID,
			RoundID:        a.RoundID,
			FromPlayerID:   a.FromPlayerID,
			ToPlayerID:     a.ToPlayerID,
			AdjustmentType: a.AdjustmentType,
			Amount:         a.Amount,
			Reason:         a.Reason,
			Status:         a.Status,
			ExpiresAt:      a.ExpiresAt,
			CreatedAt:      a.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"adjustments": result})
}

// AcceptAdjustment handles POST /api/v1/games/:game_id/adjustments/:adjustment_id/accept
// PRD §3.2 rule 5: only receiver can accept
func (h *AdjustmentHandler) AcceptAdjustment(c *gin.Context) {
	gameID, err := strconv.ParseInt(c.Param("game_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}
	adjustmentID, err := strconv.ParseInt(c.Param("adjustment_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}
	userID := middleware.GetUserID(c)

	adj, err := h.Store.GetAdjustment(adjustmentID)
	if err != nil {
		c.JSON(http.StatusNotFound, errs.ErrNotFound)
		return
	}
	if adj.GameID != gameID {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}

	// Only the to_player's user can accept (PRD §3.2 rule 5)
	toPlayer, err := h.Store.GetGamePlayer(gameID, userID)
	if err != nil || toPlayer.ID != adj.ToPlayerID {
		c.JSON(http.StatusForbidden, errs.ErrForbidden)
		return
	}

	_, err = h.Store.ResolveAdjustment(adjustmentID, "pending", "accepted", userID)
	if err != nil {
		if be, ok := err.(*errs.BizError); ok {
			c.JSON(http.StatusBadRequest, be)
			return
		}
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}

	// Update settlement timestamp if game is ended (PRD §3.2 rule 11)
	game, _ := h.Store.GetGame(gameID)
	if game != nil && game.Status == "ended" {
		_ = h.Store.UpdateSettlementTime(gameID)
	}

	// Find from player nickname for message
	players, _ := h.Store.GetGamePlayers(gameID)
	fromNickname := ""
	for _, p := range players {
		if p.ID == adj.FromPlayerID {
			fromNickname = p.NicknameSnapshot
			break
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"adjustment_id": adj.ID,
		"status":        "accepted",
		"message":       fmt.Sprintf("积分调整已生效：%s -%d 分，你 +%d 分", fromNickname, adj.Amount, adj.Amount),
	})
}

// RejectAdjustment handles POST /api/v1/games/:game_id/adjustments/:adjustment_id/reject
func (h *AdjustmentHandler) RejectAdjustment(c *gin.Context) {
	gameID, err := strconv.ParseInt(c.Param("game_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}
	adjustmentID, err := strconv.ParseInt(c.Param("adjustment_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}
	userID := middleware.GetUserID(c)

	adj, err := h.Store.GetAdjustment(adjustmentID)
	if err != nil {
		c.JSON(http.StatusNotFound, errs.ErrNotFound)
		return
	}
	if adj.GameID != gameID {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}

	toPlayer, err := h.Store.GetGamePlayer(gameID, userID)
	if err != nil || toPlayer.ID != adj.ToPlayerID {
		c.JSON(http.StatusForbidden, errs.ErrForbidden)
		return
	}

	_, err = h.Store.ResolveAdjustment(adjustmentID, "pending", "rejected", userID)
	if err != nil {
		if be, ok := err.(*errs.BizError); ok {
			c.JSON(http.StatusBadRequest, be)
			return
		}
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"adjustment_id": adj.ID,
		"status":        "rejected",
		"message":       "积分调整未生效，原积分不变",
	})
}

// CancelAdjustment handles POST /api/v1/games/:game_id/adjustments/:adjustment_id/cancel
// PRD §3.2: only the proposer can cancel
func (h *AdjustmentHandler) CancelAdjustment(c *gin.Context) {
	gameID, err := strconv.ParseInt(c.Param("game_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}
	adjustmentID, err := strconv.ParseInt(c.Param("adjustment_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}
	userID := middleware.GetUserID(c)

	adj, err := h.Store.GetAdjustment(adjustmentID)
	if err != nil {
		c.JSON(http.StatusNotFound, errs.ErrNotFound)
		return
	}
	if adj.GameID != gameID {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}

	// Only proposer can cancel
	fromPlayer, err := h.Store.GetGamePlayer(gameID, userID)
	if err != nil || fromPlayer.ID != adj.FromPlayerID {
		c.JSON(http.StatusForbidden, errs.ErrForbidden)
		return
	}

	_, err = h.Store.ResolveAdjustment(adjustmentID, "pending", "cancelled", userID)
	if err != nil {
		if be, ok := err.(*errs.BizError); ok {
			c.JSON(http.StatusBadRequest, be)
			return
		}
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"adjustment_id": adj.ID,
		"status":        "cancelled",
		"message":       "积分调整已取消，原积分不变",
	})
}
