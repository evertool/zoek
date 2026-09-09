package handler

import (
	"fmt"
	"net/http"
	"strings"
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
	GameID      int64  `json:"game_id"`
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
	PlayerID  int64     `json:"player_id"`
	UserID    int64     `json:"user_id"`
	Nickname  string    `json:"nickname"`
	AvatarURL string    `json:"avatar_url"`
	Role      string    `json:"role"`
	JoinedAt  time.Time `json:"joined_at"`
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

// defaultGameName returns the auto-generated table name, e.g. "得闲开台 9月9日".
func defaultGameName() string {
	return "得闲开台 " + strconv.Itoa(int(time.Now().Month())) + "月" + strconv.Itoa(time.Now().Day()) + "日"
}

// CreateGame handles POST /api/v1/games (PRD §4.2-A: 开桌)
func (h *GameHandler) CreateGame(c *gin.Context) {
	var req CreateGameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}
	userID := middleware.GetUserID(c)

	// 一个用户同时只能有一张进行中的牌台
	if activeID, err := h.Store.GetUserActiveGameID(userID, 0); err == nil && activeID > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"code":    errs.ErrAlreadyInGame.Code,
			"message": errs.ErrAlreadyInGame.Message,
			"action":  errs.ErrAlreadyInGame.Action,
			"game_id": activeID,
		})
		return
	}

	// PRD v1.0 §4.2-A: 开台零摩擦，不填台名，自动生成"得闲开台 M月D日"
	name := req.Name
	if name == "" {
		name = defaultGameName()
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
// 进行中的牌台（互斥规则下至多一张），附带座位玩家昵称/头像/风位；
// 不返回他人得分（PRD 开牌阶段隐私：未锁定的分不展示）。
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

		players, _ := h.Store.GetGamePlayers(g.ID)
		playerItems := make([]gin.H, 0, len(players))
		for _, p := range players {
			wind := ""
			if p.Seat >= 1 && p.Seat <= 4 {
				wind = []string{"東", "南", "西", "北"}[p.Seat-1]
			}
			avatar := ""
			if u, err := h.Store.GetUserByID(p.UserID); err == nil {
				avatar = u.AvatarURL
			}
			playerItems = append(playerItems, gin.H{
				"player_id":  p.ID,
				"user_id":    p.UserID,
				"nickname":   p.NicknameSnapshot,
				"avatar_url": avatar,
				"seat":       p.Seat,
				"wind":       wind,
				"is_me":      p.UserID == userID,
			})
		}

		var duration int
		if g.StartedAt != nil {
			duration = int(time.Since(*g.StartedAt).Minutes())
		}

		result = append(result, gin.H{
			"game_id":              g.ID,
			"name":                 g.Name,
			"status":               g.Status,
			"player_count":         count,
			"max_players":          4,
			"players":              playerItems,
			"current_round_number": roundNum,
			"completed_rounds":     completed,
			"duration_minutes":     duration,
			"started_at":           g.StartedAt,
			"created_at":           g.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"games": result})
}

// GetHistoryGames handles GET /api/v1/games/history
// 记录页列表：日期筛选(days=7/30/0) + 标签筛选(result=win/draw/lose) + 分页。
// 标签按本场名次划分：第 1 名=胜，末名=负，中间=平。
func (h *GameHandler) GetHistoryGames(c *gin.Context) {
	userID := middleware.GetUserID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	days, _ := strconv.Atoi(c.DefaultQuery("days", "0"))
	result := c.DefaultQuery("result", "")
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	if result != "" && result != "win" && result != "draw" && result != "lose" {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}

	games, err := h.Store.GetHistoryGamesAll(userID, store.HistoryGameFilters{Days: days})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}

	type histItem struct {
		game          model.Game
		myScore       int
		myRank        int
		tag           string
		rounds        int64
		players       []model.GamePlayer
		hasAdjustment bool
	}
	items := make([]histItem, 0, len(games))
	if len(games) > 0 {
		gameIDs := make([]int64, 0, len(games))
		for _, g := range games {
			gameIDs = append(gameIDs, g.ID)
		}
		playersByGame := map[int64][]model.GamePlayer{}
		if plist, err := h.Store.GetPlayersByGameIDs(gameIDs); err == nil {
			for _, p := range plist {
				playersByGame[p.GameID] = append(playersByGame[p.GameID], p)
			}
		}
		totalsByGame, _ := h.Store.GetTotalsByGameIDs(gameIDs)
		adjCount, _ := h.Store.CountAdjustmentsByGameIDs(gameIDs)
		roundCounts, _ := h.Store.CountLockedRoundsByGameIDs(gameIDs)

		for _, g := range games {
			players := playersByGame[g.ID]
			totals := totalsByGame[g.ID]
			var myGP *model.GamePlayer
			for i := range players {
				if players[i].UserID == userID {
					myGP = &players[i]
					break
				}
			}
			if myGP == nil {
				continue
			}
			myScore := totals[myGP.ID]
			myRank := 1
			for _, p := range players {
				if p.ID != myGP.ID && totals[p.ID] > myScore {
					myRank++
				}
			}
			tag := "draw"
			if len(players) >= 2 {
				if myRank == 1 {
					tag = "win"
				} else if myRank == len(players) {
					tag = "lose"
				}
			}
			if result != "" && tag != result {
				continue
			}
			items = append(items, histItem{
				game: g, myScore: myScore, myRank: myRank, tag: tag,
				rounds: roundCounts[g.ID],
				players: players, hasAdjustment: adjCount[g.ID] > 0,
			})
		}
	}

	total := len(items)

	// 概览统计基于整个筛选结果集（而非当前分页）
	var sumNet, winCount int
	for _, it := range items {
		sumNet += it.myScore
		if it.myRank == 1 {
			winCount++
		}
	}
	winRate := 0.0
	if total > 0 {
		winRate = float64(int(float64(winCount)/float64(total)*1000+0.5)) / 10
	}
	summary := gin.H{
		"games":    total,
		"net":      sumNet,
		"wins":     winCount,
		"win_rate": winRate,
	}

	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	result2 := make([]gin.H, 0, end-start)
	for _, it := range items[start:end] {
		g := it.game
		playerItems := make([]gin.H, 0, len(it.players))
		for _, p := range it.players {
			playerItems = append(playerItems, gin.H{"nickname": p.NicknameSnapshot})
		}
		var duration int
		if g.StartedAt != nil && g.EndedAt != nil {
			duration = int(g.EndedAt.Sub(*g.StartedAt).Minutes())
		}
		result2 = append(result2, gin.H{
			"game_id":          g.ID,
			"name":             g.Name,
			"status":           g.Status,
			"completed_rounds": it.rounds,
			"player_count":     len(it.players),
			"players":          playerItems,
			"my_score":         it.myScore,
			"my_rank":          it.myRank,
			"result":           it.tag,
			"is_ranked":        len(it.players) == 4,
			"has_adjustment":   it.hasAdjustment,
			"duration_minutes": duration,
			"ended_at":         g.EndedAt,
			"created_at":       g.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"games":     result2,
		"total":     total,
		"summary":   summary,
		"page":      page,
		"page_size": pageSize,
		"has_more":  end < total,
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
	// 批量获取用户信息以填充头像
	userCache := map[int64]*model.User{}
	for _, p := range players {
		var user *model.User
		if u, ok := userCache[p.UserID]; ok {
			user = u
		} else {
			user, _ = h.Store.GetUserByID(p.UserID)
			userCache[p.UserID] = user
		}
		avatarURL := ""
		if user != nil {
			avatarURL = user.AvatarURL
		}
		playerInfos = append(playerInfos, PlayerInfo{
			PlayerID:  p.ID,
			UserID:    p.UserID,
			Nickname:  p.NicknameSnapshot,
			AvatarURL: avatarURL,
			Role:      p.Role,
			JoinedAt:  p.JoinedAt,
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
	} else if numericID, convErr := strconv.ParseInt(strings.TrimSpace(req.InviteToken), 10, 64); convErr == nil && numericID > 0 {
		// 分享兜底路径：invite_token 直接传 game_id（房间页/首页分享）
		game, err = h.Store.GetGame(numericID)
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

	// 一个用户同时只能在一间房：已在别的牌台则拒绝，并附上现有牌台 ID 供前端跳转
	if activeID, err := h.Store.GetUserActiveGameID(userID, game.ID); err == nil && activeID > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"code":    errs.ErrAlreadyInGame.Code,
			"message": errs.ErrAlreadyInGame.Message,
			"action":  errs.ErrAlreadyInGame.Action,
			"game_id": activeID,
		})
		return
	}

	// 可加入状态：组桌中；或已自动开局但还没凑满 4 人（继续凑脚，满 4 后锁定）
	if game.Status != "forming" && game.Status != "active" {
		c.JSON(http.StatusBadRequest, errs.ErrMembersLocked)
		return
	}
	if game.Status == "active" && (game.MembersLocked || membersFull(game.ID, h)) {
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

	// 人够自动开局：凑满 2 人即激活牌桌并创建第 1 局（房间页不再设开始按钮）
	if game.Status == "forming" {
		_, _ = h.Store.StartGameIfReady(game.ID)
	}

	// Refresh game status after potential auto-start
	fresh, _ := h.Store.GetGame(game.ID)
	status := game.Status
	if fresh != nil {
		status = fresh.Status
	}

	c.JSON(http.StatusCreated, gin.H{
		"game_id":   game.ID,
		"player_id": player.ID,
		"status":    status,
		"message":   "加入成功",
	})
}

// SwapSeat handles POST /api/v1/games/:game_id/swap_seat（长按空位直接换座，无需申请）
func (h *GameHandler) SwapSeat(c *gin.Context) {
	gameID, err := strconv.ParseInt(c.Param("game_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}
	var req struct {
		TargetSeat int `json:"target_seat"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errs.ErrInvalidInput)
		return
	}
	userID := middleware.GetUserID(c)

	game, err := h.Store.GetGame(gameID)
	if err != nil {
		c.JSON(http.StatusNotFound, errs.ErrNotFound)
		return
	}
	if game.Status == "ended" || game.Status == "cancelled" {
		c.JSON(http.StatusBadRequest, errs.ErrGameEnded)
		return
	}

	player, err := h.Store.SwapToEmptySeat(gameID, userID, req.TargetSeat)
	if err != nil {
		if be, ok := err.(*errs.BizError); ok {
			c.JSON(http.StatusBadRequest, be)
			return
		}
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"game_id":   gameID,
		"seat":      player.Seat,
		"message":   "已换座",
	})
}

// membersFull 牌局是否已凑满 4 人。
func membersFull(gameID int64, h *GameHandler) bool {
	count, err := h.Store.CountGamePlayers(gameID)
	return err != nil || count >= 4
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

	// 人够已自动开局（JoinGame 时触发），start 幂等返回当前状态
	if game.Status == "active" {
		round, _ := h.Store.GetCurrentRound(gameID)
		completed, _ := h.Store.CountLockedRounds(gameID)
		roundNum := 0
		if round != nil {
			roundNum = round.RoundNumber
		}
		c.JSON(http.StatusOK, gin.H{
			"game_id":              gameID,
			"status":               "active",
			"current_round_number": roundNum,
			"completed_rounds":     completed,
			"message":              fmt.Sprintf("第%d局进行中", roundNum),
		})
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

	// 允许取消：组桌中，或已开局但还没有入账的局（有记分记录须走散台结算）
	completed, _ := h.Store.CountLockedRounds(gameID)
	if (game.Status != "forming" && game.Status != "active") || completed > 0 {
		c.JSON(http.StatusBadRequest, errs.ErrGameHasScores)
		return
	}

	_, err = h.Store.UpdateGameStatus(gameID, game.Status, "cancelled")
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

	// 4 人局散台即排位结算（幂等；结算失败不阻断散台响应）
	_ = h.Store.SettleGameRank(gameID)

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

	// 房间未满员即可生成邀请 QR（组桌中和记分中都可以；锁员/结束后失效）
	if game.Status != "forming" && game.Status != "active" {
		c.JSON(http.StatusBadRequest, errs.ErrGameNotForming)
		return
	}
	if game.MembersLocked {
		c.JSON(http.StatusBadRequest, errs.ErrMembersLocked)
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

// HideGame handles POST /api/v1/games/:game_id/hide — 用户从自己的对局记录中
// 删除该场牌局（仅对自己隐藏，不影响其他参与者和原始记录）。
func (h *GameHandler) HideGame(c *gin.Context) {
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

	// Must be a participant
	if _, pErr := h.Store.GetGamePlayer(gameID, userID); pErr != nil {
		c.JSON(http.StatusForbidden, errs.ErrForbidden)
		return
	}

	// 只有已结束/失效/取消的牌局可以从记录中删除
	if game.Status != "ended" && game.Status != "expired" && game.Status != "cancelled" {
		c.JSON(http.StatusBadRequest, errs.New("GAME_NOT_FINISHED", "进行中的牌局不能删除", errs.ActionRefreshGame))
		return
	}

	if err := h.Store.HideGame(userID, gameID); err != nil {
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}

	c.JSON(http.StatusOK, SimpleResponse{Message: "已从对局记录删除"})
}
