package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lk/zoek/backend/internal/errs"
	"github.com/lk/zoek/backend/internal/middleware"
	"github.com/lk/zoek/backend/internal/store"
)

// SettlementHandler handles settlement and history queries.
type SettlementHandler struct {
	Store *store.Store
}

func NewSettlementHandler(s *store.Store) *SettlementHandler {
	return &SettlementHandler{Store: s}
}

// ---------------------------------------------------------------------------
// Response types
// ---------------------------------------------------------------------------

type SettlementPlayer struct {
	PlayerID    int64  `json:"player_id"`
	Nickname    string `json:"nickname"`
	TotalScore  int64  `json:"total_score"`
	Rank        int    `json:"rank"`
	Adjustments int    `json:"adjustments"`
}

type SettlementResponse struct {
	GameID              int64              `json:"game_id"`
	GameName            string             `json:"game_name"`
	CompletedRounds     int                `json:"completed_rounds"`
	CurrentRoundNumber  *int               `json:"current_round_number"`
	AdjustmentCount     int                `json:"adjustment_count"`
	SettlementUpdatedAt *string            `json:"settlement_updated_at,omitempty"`
	Players             []SettlementPlayer `json:"players"`
}

type HistoryDetailResponse struct {
	GameID          int64                `json:"game_id"`
	GameName        string               `json:"game_name"`
	Status          string               `json:"status"`
	CompletedRounds int                  `json:"completed_rounds"`
	CreatedAt       string               `json:"created_at"`
	EndedAt         *string              `json:"ended_at,omitempty"`
	Rounds          []HistoryRoundDetail `json:"rounds"`
	Adjustments     []AdjustmentResponse `json:"adjustments"`
}

type HistoryRoundDetail struct {
	RoundID     int64              `json:"round_id"`
	RoundNumber int                `json:"round_number"`
	Status      string             `json:"status"`
	Submissions []SubmissionDetail `json:"submissions"`
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// GetSettlement handles GET /api/v1/games/:game_id/settlement (PRD §3.5)
func (h *SettlementHandler) GetSettlement(c *gin.Context) {
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

	_, pErr := h.Store.GetGamePlayer(gameID, userID)
	if pErr != nil {
		c.JSON(http.StatusForbidden, errs.ErrForbidden)
		return
	}

	// Aggregate settlement
	totals, adjCount, err := h.Store.AggregateSettlement(gameID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}

	completed, _ := h.Store.CountLockedRounds(gameID)

	round, _ := h.Store.GetCurrentRound(gameID)
	var roundNum *int
	if round != nil && game.Status == "active" {
		n := round.RoundNumber
		roundNum = &n
	}

	players := make([]SettlementPlayer, 0, len(totals))
	for _, t := range totals {
		players = append(players, SettlementPlayer{
			PlayerID:    t.PlayerID,
			Nickname:    t.Nickname,
			TotalScore:  t.TotalScore,
			Rank:        t.Rank,
			Adjustments: t.Adjustments,
		})
	}

	var settlementUpdatedAt *string
	if game.SettlementUpdatedAt != nil {
		s := game.SettlementUpdatedAt.Format("2006-01-02T15:04:05Z07:00")
		settlementUpdatedAt = &s
	}

	c.JSON(http.StatusOK, SettlementResponse{
		GameID:              game.ID,
		GameName:            game.Name,
		CompletedRounds:     completed,
		CurrentRoundNumber:  roundNum,
		AdjustmentCount:     adjCount,
		SettlementUpdatedAt: settlementUpdatedAt,
		Players:             players,
	})
}

// GetHistoryDetail handles GET /api/v1/games/:game_id/history (PRD §3.5)
func (h *SettlementHandler) GetHistoryDetail(c *gin.Context) {
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

	_, pErr := h.Store.GetGamePlayer(gameID, userID)
	if pErr != nil {
		c.JSON(http.StatusForbidden, errs.ErrForbidden)
		return
	}

	// Get all rounds
	rounds, err := h.Store.GetRoundsByGameID(gameID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}

	players, _ := h.Store.GetGamePlayers(gameID)
	playerMap := make(map[int64]PlayerInfo)
	for _, p := range players {
		playerMap[p.ID] = PlayerInfo{
			PlayerID: p.ID,
			UserID:   p.UserID,
			Nickname: p.NicknameSnapshot,
		}
	}

	historyRounds := make([]HistoryRoundDetail, 0, len(rounds))
	for _, r := range rounds {
		subs, _ := h.Store.GetSubmissions(r.ID)
		subDetails := make([]SubmissionDetail, 0, len(subs))
		for _, s := range subs {
			info, ok := playerMap[s.GamePlayerID]
			if ok {
				subDetails = append(subDetails, SubmissionDetail{
					PlayerID:  info.PlayerID,
					UserID:    info.UserID,
					Nickname:  info.Nickname,
					Score:     s.Score,
					Submitted: true,
				})
			}
		}
		historyRounds = append(historyRounds, HistoryRoundDetail{
			RoundID:     r.ID,
			RoundNumber: r.RoundNumber,
			Status:      r.Status,
			Submissions: subDetails,
		})
	}

	// Get adjustments
	adjs, err := h.Store.GetAdjustmentsByGameID(gameID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}
	adjResponses := make([]AdjustmentResponse, 0, len(adjs))
	for _, a := range adjs {
		adjResponses = append(adjResponses, AdjustmentResponse{
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

	var endedAt *string
	if game.EndedAt != nil {
		s := game.EndedAt.Format("2006-01-02T15:04:05Z07:00")
		endedAt = &s
	}

	c.JSON(http.StatusOK, HistoryDetailResponse{
		GameID:          game.ID,
		GameName:        game.Name,
		Status:          game.Status,
		CompletedRounds: len(historyRounds),
		CreatedAt:       game.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		EndedAt:         endedAt,
		Rounds:          historyRounds,
		Adjustments:     adjResponses,
	})
}
