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

// RoundHandler handles round and submission operations.
type RoundHandler struct {
	Store *store.Store
}

func NewRoundHandler(s *store.Store) *RoundHandler {
	return &RoundHandler{Store: s}
}

// ---------------------------------------------------------------------------
// Request/Response types
// ---------------------------------------------------------------------------

type SubmitScoreRequest struct {
	Score     int    `json:"score"     binding:"required"`
	RequestID string `json:"request_id"`
}

type SubmitScoreResponse struct {
	RoundID            int64  `json:"round_id"`
	RoundNumber        int    `json:"round_number"`
	RoundStatus        string `json:"round_status"`
	MyScore            int    `json:"my_score"`
	MySubmitted        bool   `json:"my_submitted"`
	SubmittedCount     int    `json:"submitted_count"`
	MemberCount        int    `json:"member_count"`
	CompletedRounds    int    `json:"completed_rounds"`
	ScoreSum           int64  `json:"score_sum"`
	CurrentRoundNumber int    `json:"current_round_number"`
	Message            string `json:"message"`
}

type RoundDetailResponse struct {
	RoundID            int64              `json:"round_id"`
	RoundNumber        int                `json:"round_number"`
	Status             string             `json:"status"`
	SubmittedCount     int                `json:"submitted_count"`
	MemberCount        int                `json:"member_count"`
	ScoreSum           int64              `json:"score_sum,omitempty"`
	CompletedRounds    int                `json:"completed_rounds"`
	CurrentRoundNumber int                `json:"current_round_number"`
	Submissions        []SubmissionDetail `json:"submissions,omitempty"`
}

type SubmissionDetail struct {
	PlayerID  int64  `json:"player_id"`
	UserID    int64  `json:"user_id"`
	Nickname  string `json:"nickname"`
	Score     int    `json:"score"`
	Submitted bool   `json:"submitted"`
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// requireGamePlayer checks that the user is a player in the game and returns the player.
func (h *RoundHandler) requireGamePlayer(gameID, userID int64) (*model.GamePlayer, error) {
	player, err := h.Store.GetGamePlayer(gameID, userID)
	if err != nil {
		return nil, errs.ErrForbidden
	}
	return player, nil
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// CreateNextRound handles POST /api/v1/games/:game_id/rounds (PRD §2.2 rules 7-8: 创建下一局)
// and POST /api/v1/games/:game_id/rounds/:round_id/next (idempotent next round)
func (h *RoundHandler) CreateNextRound(c *gin.Context) {
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

	// Must be a player
	_, pErr := h.requireGamePlayer(gameID, userID)
	if pErr != nil {
		c.JSON(http.StatusForbidden, pErr)
		return
	}

	// Get current round
	currentRound, err := h.Store.GetCurrentRound(gameID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}

	if currentRound == nil {
		c.JSON(http.StatusBadRequest, errs.ErrGameNotActive)
		return
	}

	// Idempotent: if current round is already open (created by a previous
	// concurrent request), return it directly (PRD §2.2 rule 8).
	if currentRound.Status == "open" {
		completed, _ := h.Store.CountLockedRounds(gameID)
		c.JSON(http.StatusOK, gin.H{
			"round_id":             currentRound.ID,
			"round_number":         currentRound.RoundNumber,
			"status":               currentRound.Status,
			"completed_rounds":     completed,
			"current_round_number": currentRound.RoundNumber,
			"message":              fmt.Sprintf("第%d局进行中", currentRound.RoundNumber),
		})
		return
	}

	if currentRound.Status != "ready_for_next" {
		c.JSON(http.StatusBadRequest, errs.New("ROUND_NOT_READY", "当前局未完成，不能创建下一局", errs.ActionRefreshGame))
		return
	}

	// Create next round (idempotent via unique constraint)
	nextNum := currentRound.RoundNumber + 1
	round, err := h.Store.CreateRound(gameID, nextNum)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}

	completed, _ := h.Store.CountLockedRounds(gameID)

	c.JSON(http.StatusCreated, gin.H{
		"round_id":             round.ID,
		"round_number":         round.RoundNumber,
		"status":               round.Status,
		"completed_rounds":     completed,
		"current_round_number": round.RoundNumber,
		"message":              fmt.Sprintf("第%d局开始，请提交本局积分", round.RoundNumber),
	})
}

// GetCurrentRound handles GET /api/v1/games/:game_id/rounds/current
func (h *RoundHandler) GetCurrentRound(c *gin.Context) {
	gameID, err := strconv.ParseInt(c.Param("game_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}
	userID := middleware.GetUserID(c)

	_, pErr := h.requireGamePlayer(gameID, userID)
	if pErr != nil {
		c.JSON(http.StatusForbidden, pErr)
		return
	}

	round, err := h.Store.GetCurrentRound(gameID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}
	if round == nil {
		c.JSON(http.StatusOK, gin.H{
			"current_round_number": nil,
			"completed_rounds":     0,
		})
		return
	}

	players, _ := h.Store.GetGamePlayers(gameID)
	subs, _ := h.Store.GetSubmissions(round.ID)
	completed, _ := h.Store.CountLockedRounds(gameID)

	// Build submission map
	subMap := make(map[int64]int) // game_player_id -> score
	submitted := make(map[int64]bool)
	var scoreSum int64
	for _, s := range subs {
		subMap[s.GamePlayerID] = s.Score
		submitted[s.GamePlayerID] = true
		scoreSum += int64(s.Score)
	}

	submissionDetails := make([]SubmissionDetail, 0, len(players))
	for _, p := range players {
		sd := SubmissionDetail{
			PlayerID:  p.ID,
			UserID:    p.UserID,
			Nickname:  p.NicknameSnapshot,
			Submitted: submitted[p.ID],
			Score:     subMap[p.ID],
		}
		submissionDetails = append(submissionDetails, sd)
	}

	// In open phase, don't show other players' scores (PRD §2.3 rule 5)
	if round.Status == "open" {
		for i := range submissionDetails {
			if submissionDetails[i].UserID != userID {
				submissionDetails[i].Score = 0
			}
		}
		// Don't show sum in open phase
		scoreSum = 0
	}

	roundNum := round.RoundNumber
	c.JSON(http.StatusOK, RoundDetailResponse{
		RoundID:            round.ID,
		RoundNumber:        round.RoundNumber,
		Status:             round.Status,
		SubmittedCount:     len(subs),
		MemberCount:        len(players),
		ScoreSum:           scoreSum,
		CompletedRounds:    completed,
		CurrentRoundNumber: roundNum,
		Submissions:        submissionDetails,
	})
}

// SubmitScore handles PUT /api/v1/games/:game_id/rounds/:round_id/submission (PRD §2.3)
func (h *RoundHandler) SubmitScore(c *gin.Context) {
	gameID, err := strconv.ParseInt(c.Param("game_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}
	roundID, err := strconv.ParseInt(c.Param("round_id"), 10, 64)
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

	round, err := h.Store.GetRound(roundID)
	if err != nil {
		c.JSON(http.StatusNotFound, errs.ErrNotFound)
		return
	}
	if round.GameID != gameID {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}

	// Round must be open or review (allow modification in review per PRD §2.2 rule 4)
	if round.Status != "open" && round.Status != "review" {
		c.JSON(http.StatusBadRequest, errs.New("ROUND_LOCKED", "本局已锁定，不能修改", errs.ActionRefreshGame))
		return
	}

	player, pErr := h.requireGamePlayer(gameID, userID)
	if pErr != nil {
		c.JSON(http.StatusForbidden, pErr)
		return
	}

	var req SubmitScoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}

	requestID := req.RequestID
	if requestID == "" {
		requestID = middleware.GetRequestID(c)
	}
	if requestID == "" {
		requestID = fmt.Sprintf("auto-%d-%d-%d", gameID, roundID, time.Now().UnixNano())
	}

	// Upsert submission
	_, err = h.Store.UpsertSubmission(roundID, player.ID, req.Score, requestID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}

	// Lock members on first round first submission (PRD §2.1 rule 4)
	if round.RoundNumber == 1 && !game.MembersLocked {
		_ = h.Store.LockMembers(gameID)
	}

	// Check if all submitted
	players, _ := h.Store.GetGamePlayers(gameID)
	subs, _ := h.Store.GetSubmissions(roundID)

	allSubmitted := len(subs) >= len(players)

	// If all submitted and round was open, transition to review
	if allSubmitted && round.Status == "open" {
		_, _ = h.Store.UpdateRoundStatus(roundID, "open", "review")
		round.Status = "review"
	}

	// Recalculate sum
	var scoreSum int64
	for _, s := range subs {
		scoreSum += int64(s.Score)
	}
	// Include the just-submitted score if it's a new submission
	mySub, _ := h.Store.GetSubmission(roundID, player.ID)
	if mySub != nil {
		// Recalculate from all subs (the upsert already updated)
		subs, _ = h.Store.GetSubmissions(roundID)
		scoreSum = 0
		for _, s := range subs {
			scoreSum += int64(s.Score)
		}
	}

	completed, _ := h.Store.CountLockedRounds(gameID)

	// Build message
	var message string
	if allSubmitted && round.Status == "review" {
		message = fmt.Sprintf("第%d局已提交完成，请核对本局分数", round.RoundNumber)
	} else if req.Score > 0 {
		message = fmt.Sprintf("已提交本局 +%d 分，等待其他玩家提交", req.Score)
	} else if req.Score < 0 {
		message = fmt.Sprintf("已提交本局 %d 分（本局扣除 %d 分）", req.Score, -req.Score)
	} else {
		message = "已提交本局 0 分"
	}

	roundNum := round.RoundNumber
	c.JSON(http.StatusOK, SubmitScoreResponse{
		RoundID:            round.ID,
		RoundNumber:        round.RoundNumber,
		RoundStatus:        round.Status,
		MyScore:            req.Score,
		MySubmitted:        true,
		SubmittedCount:     len(subs),
		MemberCount:        len(players),
		CompletedRounds:    completed,
		ScoreSum:           scoreSum,
		CurrentRoundNumber: roundNum,
		Message:            message,
	})
}

// LockRound handles POST /api/v1/games/:game_id/rounds/:round_id/lock (PRD §2.2 rules 5-6)
func (h *RoundHandler) LockRound(c *gin.Context) {
	gameID, err := strconv.ParseInt(c.Param("game_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}
	roundID, err := strconv.ParseInt(c.Param("round_id"), 10, 64)
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

	round, err := h.Store.GetRound(roundID)
	if err != nil {
		c.JSON(http.StatusNotFound, errs.ErrNotFound)
		return
	}

	_, pErr := h.requireGamePlayer(gameID, userID)
	if pErr != nil {
		c.JSON(http.StatusForbidden, pErr)
		return
	}

	if round.Status != "review" {
		c.JSON(http.StatusBadRequest, errs.ErrRoundNotReview)
		return
	}

	// Verify zero sum (PRD §2.2 rule 6, §2.3 rule 7)
	subs, err := h.Store.GetSubmissions(roundID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}
	players, err := h.Store.GetGamePlayers(gameID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}

	// Check all submitted
	if len(subs) < len(players) {
		c.JSON(http.StatusBadRequest, errs.ErrRoundNotReady)
		return
	}

	var scoreSum int64
	for _, s := range subs {
		scoreSum += int64(s.Score)
	}
	if scoreSum != 0 {
		c.JSON(http.StatusBadRequest, errs.New("ZERO_SUM_FAILED",
			fmt.Sprintf("本局总和为 %d，请检查并修改", scoreSum), errs.ActionRetry))
		return
	}

	// Lock the round (review → ready_for_next)
	_, err = h.Store.UpdateRoundStatus(roundID, "review", "ready_for_next")
	if err != nil {
		c.JSON(http.StatusBadRequest, errs.New("ROUND_LOCKED", "本局已锁定", errs.ActionRefreshGame))
		return
	}

	completed, _ := h.Store.CountLockedRounds(gameID)
	roundNum := round.RoundNumber

	c.JSON(http.StatusOK, gin.H{
		"round_id":             round.ID,
		"round_number":         round.RoundNumber,
		"status":               "ready_for_next",
		"completed_rounds":     completed,
		"current_round_number": roundNum,
		"message":              fmt.Sprintf("第%d局已完成，本局总和为0", round.RoundNumber),
	})
}

// ManualNextRound handles POST /api/v1/games/:game_id/rounds/manual-next
// 手动「开下一局」：任何在桌玩家可触发，结束当前局并开新局（免锁定的局边界标记）。
// 台间转分恒为 0 和，局边界只能由人标记；旧版逐人提交未配平时拒绝切局。
func (h *RoundHandler) ManualNextRound(c *gin.Context) {
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
	if _, pErr := h.requireGamePlayer(gameID, userID); pErr != nil {
		c.JSON(http.StatusForbidden, pErr)
		return
	}

	round, err := h.Store.GetCurrentRound(gameID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}
	if round == nil {
		// 尚无任何局：直接开第 1 局
		created, cErr := h.Store.CreateRound(gameID, 1)
		if cErr != nil {
			c.JSON(http.StatusInternalServerError, errs.ErrInternal)
			return
		}
		c.JSON(http.StatusOK, gin.H{"round_id": created.ID, "round_number": created.RoundNumber, "message": "已开始第 1 局"})
		return
	}

	switch round.Status {
	case "open", "review":
		subs, _ := h.Store.GetSubmissions(round.ID)
		var sum int64
		for _, sb := range subs {
			sum += int64(sb.Score)
		}
		if len(subs) > 0 && sum != 0 {
			c.JSON(http.StatusBadRequest, errs.New("ROUND_INCOMPLETE", "本局总分不为 0，请核对补记后再开下一局", errs.ActionRefreshGame))
			return
		}
		if _, uErr := h.Store.UpdateRoundStatus(round.ID, round.Status, "ready_for_next"); uErr != nil {
			c.JSON(http.StatusInternalServerError, errs.ErrInternal)
			return
		}
	case "ready_for_next", "locked":
		// 已收尾，直接开下一局
	default:
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}

	next, err := h.Store.CreateRound(gameID, round.RoundNumber+1)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"round_id":      next.ID,
		"round_number":  next.RoundNumber,
		"message":       fmt.Sprintf("已开始第 %d 局", next.RoundNumber),
	})
}

// GetRoundDetail handles GET /api/v1/games/:game_id/rounds/:round_id
func (h *RoundHandler) GetRoundDetail(c *gin.Context) {
	gameID, err := strconv.ParseInt(c.Param("game_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}
	roundID, err := strconv.ParseInt(c.Param("round_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}
	userID := middleware.GetUserID(c)

	_, pErr := h.requireGamePlayer(gameID, userID)
	if pErr != nil {
		c.JSON(http.StatusForbidden, pErr)
		return
	}

	round, err := h.Store.GetRound(roundID)
	if err != nil {
		c.JSON(http.StatusNotFound, errs.ErrNotFound)
		return
	}
	if round.GameID != gameID {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}

	players, _ := h.Store.GetGamePlayers(gameID)
	subs, _ := h.Store.GetSubmissions(roundID)
	completed, _ := h.Store.CountLockedRounds(gameID)

	subMap := make(map[int64]int)
	submitted := make(map[int64]bool)
	var scoreSum int64
	for _, s := range subs {
		subMap[s.GamePlayerID] = s.Score
		submitted[s.GamePlayerID] = true
		scoreSum += int64(s.Score)
	}

	submissionDetails := make([]SubmissionDetail, 0, len(players))
	for _, p := range players {
		sd := SubmissionDetail{
			PlayerID:  p.ID,
			UserID:    p.UserID,
			Nickname:  p.NicknameSnapshot,
			Submitted: submitted[p.ID],
			Score:     subMap[p.ID],
		}
		submissionDetails = append(submissionDetails, sd)
	}

	// Hide other scores in open phase (PRD §2.3 rule 5)
	if round.Status == "open" {
		for i := range submissionDetails {
			if submissionDetails[i].UserID != userID {
				submissionDetails[i].Score = 0
			}
		}
		scoreSum = 0
	}

	roundNum := round.RoundNumber
	c.JSON(http.StatusOK, RoundDetailResponse{
		RoundID:            round.ID,
		RoundNumber:        round.RoundNumber,
		Status:             round.Status,
		SubmittedCount:     len(subs),
		MemberCount:        len(players),
		ScoreSum:           scoreSum,
		CompletedRounds:    completed,
		CurrentRoundNumber: roundNum,
		Submissions:        submissionDetails,
	})
}
