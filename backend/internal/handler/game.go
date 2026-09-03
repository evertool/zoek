package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lk/zoek/backend/internal/errs"
	"github.com/lk/zoek/backend/internal/middleware"
	"github.com/lk/zoek/backend/internal/model"
	"github.com/lk/zoek/backend/internal/store"
	"github.com/lk/zoek/backend/pkg/wechat"
)

// GameHandler handles game table operations.
type GameHandler struct {
	Store      *store.Store
	JWTManager *middleware.JWTManager
	WxClient   *wechat.Client
}

func NewGameHandler(s *store.Store, jwt *middleware.JWTManager, wx *wechat.Client) *GameHandler {
	return &GameHandler{Store: s, JWTManager: jwt, WxClient: wx}
}

// ---------------------------------------------------------------------------
// Request/Response types
// ---------------------------------------------------------------------------

type CreateGameRequest struct {
	Name      string `json:"name"`
	RequestID string `json:"request_id"`
}

type CreateGameResponse struct {
	GameID      int64  `json:"game_id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	InviteToken string `json:"invite_token"`
	PlayerCount int    `json:"player_count"`
	CreatedAt   string `json:"created_at"`
}

type JoinGameRequest struct {
	InviteToken string `json:"invite_token"`
	GameID     int64  `json:"game_id"`
	Nickname    string `json:"nickname"`
	RequestID   string `json:"request_id"`
}

type GameDetailResponse struct {
	GameID             int64        `json:"game_id"`
	Name               string       `json:"name"`
	Status             string       `json:"status"`
	CreatorID          int64        `json:"creator_id"`
	PlayerCount        int          `json:"player_count"`
	MaxPlayers         int          `json:"max_players"`
	MembersLocked      bool         `json:"members_locked"`
	CurrentRoundNumber *int         `json:"current_round_number"`
	CompletedRounds    int          `json:"completed_rounds"`
	StartedAt          *time.Time   `json:"started_at,omitempty"`
	EndedAt            *time.Time   `json:"ended_at,omitempty"`
	CreatedAt          time.Time    `json:"created_at"`
	Players            []PlayerInfo `json:"players"`
}

type PlayerInfo struct {
	PlayerID int64     `json:"player_id"`
	UserID   int64     `json:"user_id"`
	Nickname string    `json:"nickname"`
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

type StartGameRequest struct {
	RequestID string `json:"request_id"`
}

type SimpleResponse struct {
	Message string `json:"message"`
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// CreateGame handles POST /api/v1/games (PRD §4.2-A: 开桌)
func (h *GameHandler) CreateGame(c *gin.Context) {
	var req CreateGameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}
	userID := middleware.GetUserID(c)

	name := req.Name
	if name == "" {
		name = "未命名牌局"
	}

	inviteToken := uuid.New().String()
	game, err := h.Store.CreateGame(userID, name, inviteToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}

	c.JSON(http.StatusCreated, CreateGameResponse{
		GameID:      game.ID,
		Name:        game.Name,
		Status:      game.Status,
		InviteToken: inviteToken,
		PlayerCount: 1,
		CreatedAt:   game.CreatedAt.Format(time.RFC3339),
	})
}

// GetActiveGames handles GET /api/v1/games/active
func (h *GameHandler) GetActiveGames(c *gin.Context) {
	userID := middleware.GetUserID(c)
	games, err := h.Store.GetActiveGames(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}
	result := make([]gin.H, 0, len(games))
	for _, g := range games {
		count, _ := h.Store.CountGamePlayers(g.ID)
		round, _ := h.Store.GetCurrentRound(g.ID)
		var roundNum *int
		if round != nil {
			n := round.RoundNumber
			roundNum = &n
		}
		completed, _ := h.Store.CountLockedRounds(g.ID)
		result = append(result, gin.H{
			"game_id":              g.ID,
			"name":                 g.Name,
			"status":               g.Status,
			"player_count":         count,
			"current_round_number": roundNum,
			"completed_rounds":     completed,
			"started_at":           g.StartedAt,
			"created_at":           g.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"games": result})
}

// GetHistoryGames handles GET /api/v1/games/history
func (h *GameHandler) GetHistoryGames(c *gin.Context) {
	userID := middleware.GetUserID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	games, total, err := h.Store.GetHistoryGames(userID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}
	result := make([]gin.H, 0, len(games))
	for _, g := range games {
		completed, _ := h.Store.CountLockedRounds(g.ID)
		result = append(result, gin.H{
			"game_id":          g.ID,
			"name":             g.Name,
			"status":           g.Status,
			"completed_rounds": completed,
			"ended_at":         g.EndedAt,
			"created_at":       g.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"games":     result,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetGame handles GET /api/v1/games/:game_id
func (h *GameHandler) GetGame(c *gin.Context) {
	gameID, err := strconv.ParseInt(c.Param("game_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}
	userID := middleware.GetUserID(c)

	game, err := h.Store.GetGame(gameID)
	if err != nil {
		status := http.StatusNotFound
		if be, ok := err.(*errs.BizError); ok {
			c.JSON(status, be)
			return
		}
		c.JSON(status, errs.ErrNotFound)
		return
	}

	// Check user is a player
	_, pErr := h.Store.GetGamePlayer(gameID, userID)
	if pErr != nil {
		c.JSON(http.StatusForbidden, errs.ErrForbidden)
		return
	}

	players, _ := h.Store.GetGamePlayers(gameID)
	round, _ := h.Store.GetCurrentRound(gameID)
	var roundNum *int
	if round != nil {
		n := round.RoundNumber
		roundNum = &n
	}
	completed, _ := h.Store.CountLockedRounds(gameID)

	playerInfos := make([]PlayerInfo, 0, len(players))
	for _, p := range players {
		playerInfos = append(playerInfos, PlayerInfo{
			PlayerID: p.ID,
			UserID:   p.UserID,
			Nickname: p.NicknameSnapshot,
			Role:     p.Role,
			JoinedAt: p.JoinedAt,
		})
	}

	c.JSON(http.StatusOK, GameDetailResponse{
		GameID:             game.ID,
		Name:               game.Name,
		Status:             game.Status,
		CreatorID:          game.CreatorID,
		PlayerCount:        len(players),
		MaxPlayers:         4,
		MembersLocked:      game.MembersLocked,
		CurrentRoundNumber: roundNum,
		CompletedRounds:    completed,
		StartedAt:          game.StartedAt,
		EndedAt:            game.EndedAt,
		CreatedAt:          game.CreatedAt,
		Players:            playerInfos,
	})
}

// JoinGame handles POST /api/v1/games/join (PRD §4.2-A: 扫码入桌)
func (h *GameHandler) JoinGame(c *gin.Context) {
	var req JoinGameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}
	userID := middleware.GetUserID(c)

	var game *model.Game
	var err error
	if req.GameID > 0 {
		// 从小程序码扫码进入，直接用 game_id
		game, err = h.Store.GetGame(req.GameID)
	} else {
		// 从分享链接进入，用 invite_token
		game, err = h.Store.GetGameByInviteToken(req.InviteToken)
	}
	if err != nil {
		if be, ok := err.(*errs.BizError); ok {
			c.JSON(http.StatusBadRequest, be)
			return
		}
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}

	// Check if already a player (handles re-join for any status)
	existing, _ := h.Store.GetGamePlayer(game.ID, userID)
	if existing != nil {
		c.JSON(http.StatusOK, gin.H{
			"game_id":   game.ID,
			"message":   "已加入牌桌",
			"player_id": existing.ID,
		})
		return
	}

	// Check game is forming
	if game.Status != "forming" {
		c.JSON(http.StatusBadRequest, errs.ErrMembersLocked)
		return
	}

	// Check join expiry
	if game.JoinExpiresAt != nil && time.Now().After(*game.JoinExpiresAt) {
		c.JSON(http.StatusBadRequest, errs.ErrInviteInvalid)
		return
	}

	// Get user nickname
	user, _ := h.Store.GetUserByID(userID)
	nickname := req.Nickname
	if nickname == "" {
		nickname = user.Nickname
	}

	player, err := h.Store.JoinGame(game.ID, userID, nickname)
	if err != nil {
		if be, ok := err.(*errs.BizError); ok {
			c.JSON(http.StatusBadRequest, be)
			return
		}
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"game_id":   game.ID,
		"player_id": player.ID,
		"message":   "加入成功",
	})
}

// StartGame handles POST /api/v1/games/:game_id/start (PRD §4.2-A: 开始记分)
func (h *GameHandler) StartGame(c *gin.Context) {
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

	// Only creator can start
	if game.CreatorID != userID {
		c.JSON(http.StatusForbidden, errs.ErrForbidden)
		return
	}

	if game.Status != "forming" {
		c.JSON(http.StatusBadRequest, errs.ErrGameNotForming)
		return
	}

	count, err := h.Store.CountGamePlayers(gameID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}
	if count < 2 {
		c.JSON(http.StatusBadRequest, errs.New("NOT_ENOUGH_PLAYERS", "至少需要2人才能开始", errs.ActionRetry))
		return
	}

	_, err = h.Store.UpdateGameStatus(gameID, "forming", "active")
	if err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrGameNotForming)
		return
	}

	// Create round 1
	round, err := h.Store.CreateRound(gameID, 1)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"game_id":              gameID,
		"status":               "active",
		"current_round_number": round.RoundNumber,
		"completed_rounds":     0,
		"message":              "第1局开始，请提交本局积分",
	})
}

// CancelGame handles POST /api/v1/games/:game_id/cancel
func (h *GameHandler) CancelGame(c *gin.Context) {
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

	if game.CreatorID != userID {
		c.JSON(http.StatusForbidden, errs.ErrForbidden)
		return
	}

	if game.Status != "forming" {
		c.JSON(http.StatusBadRequest, errs.ErrGameNotForming)
		return
	}

	_, err = h.Store.UpdateGameStatus(gameID, "forming", "cancelled")
	if err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrGameNotForming)
		return
	}

	c.JSON(http.StatusOK, SimpleResponse{Message: "牌桌已取消"})
}

// EndGame handles POST /api/v1/games/:game_id/end (PRD §4.2-C: 结束牌局)
func (h *GameHandler) EndGame(c *gin.Context) {
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

	if game.CreatorID != userID {
		c.JSON(http.StatusForbidden, errs.ErrForbidden)
		return
	}

	if game.Status != "active" {
		c.JSON(http.StatusBadRequest, errs.ErrGameNotActive)
		return
	}

	// Check no open/review rounds
	round, err := h.Store.GetCurrentRound(gameID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}
	if round != nil && (round.Status == "open" || round.Status == "review") {
		c.JSON(http.StatusBadRequest, errs.New("ROUND_INCOMPLETE", "还有未完成的局，完成后才能结束", errs.ActionRefreshGame))
		return
	}

	// Check at least 1 completed round
	completed, _ := h.Store.CountLockedRounds(gameID)
	if completed == 0 {
		c.JSON(http.StatusBadRequest, errs.New("NO_COMPLETED_ROUNDS", "没有已完成的局，无法结算", errs.ActionRetry))
		return
	}

	_, err = h.Store.UpdateGameStatus(gameID, "active", "ended")
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}

	_ = h.Store.InvalidateJoinExpiresAt(gameID)

	c.JSON(http.StatusOK, SimpleResponse{Message: "牌局已结束"})
}

// GetGameQRCode handles GET /api/v1/games/:game_id/qrcode
// Returns a mini program QR code image (PNG) for inviting players.
func (h *GameHandler) GetGameQRCode(c *gin.Context) {
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

	// Check user is a player
	_, pErr := h.Store.GetGamePlayer(gameID, userID)
	if pErr != nil {
		c.JSON(http.StatusForbidden, errs.ErrForbidden)
		return
	}

	// Only generating QR for forming games
	if game.Status != "forming" {
		c.JSON(http.StatusBadRequest, errs.ErrGameNotForming)
		return
	}

	// Generate QR code with scene = game_id
	scene := strconv.FormatInt(gameID, 10)
	pngData, err := h.WxClient.GetMiniProgramCode("pages/join/join", scene)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.New("QR_FAILED", "生成小程序码失败", errs.ActionRetry))
		return
	}

	c.Data(http.StatusOK, "image/png", pngData)
}
