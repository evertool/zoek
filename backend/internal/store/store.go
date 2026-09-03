package store

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/lk/zoek/backend/internal/errs"
	"github.com/lk/zoek/backend/internal/logger"
	"github.com/lk/zoek/backend/internal/model"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Store provides all database operations for the zoek backend.
type Store struct {
	DB  *gorm.DB
	log *logger.Logger
}

// New creates a Store with the given GORM DB and logger.
func New(db *gorm.DB, log *logger.Logger) *Store {
	if log == nil {
		log = logger.NewNop()
	}
	return &Store{DB: db, log: log}
}

// NewFromConfig creates a Store from database config.
func NewFromConfig(driver, dsn, logLevel string, log *logger.Logger) (*Store, error) {
	if log == nil {
		log = logger.NewNop()
	}

	gormConfig := &gorm.Config{
		Logger: gormlogger.New(
			log.NewGORMWriter(),
			gormlogger.Config{
				SlowThreshold:             200 * time.Millisecond,
				LogLevel:                  gormlogger.LogLevel(logger.GORMLogLevel(logLevel)),
				IgnoreRecordNotFoundError: true,
				Colorful:                  false,
			},
		),
	}

	// MySQL is the only supported database (PRD §7.1)
	db, err := gorm.Open(mysql.Open(dsn), gormConfig)
	_ = driver // kept for API compatibility
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}

	// Ping immediately to catch connection errors (gorm.Open is lazy for MySQL)
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get *sql.DB: %w", err)
	}
	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("ping database failed (driver=%s dsn=%s): %w", driver, dsn, err)
	}

	return &Store{DB: db, log: log}, nil
}

// AutoMigrate runs GORM auto-migration for all models.
func (s *Store) AutoMigrate() error {
	return s.DB.AutoMigrate(
		&model.User{},
		&model.Game{},
		&model.GamePlayer{},
		&model.Round{},
		&model.RoundSubmission{},
		&model.ScoreAdjustment{},
	)
}

// ===========================================================================
// User operations
// ===========================================================================

// FindOrCreateUser finds a user by openid, or creates a new one.
func (s *Store) FindOrCreateUser(openid, nickname, avatarURL string) (*model.User, error) {
	var user model.User
	err := s.DB.Where("openid = ?", openid).First(&user).Error
	if err == nil {
		return &user, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("query user: %w", err)
	}
	user = model.User{
		OpenID:    openid,
		Nickname:  nickname,
		AvatarURL: avatarURL,
	}
	if err := s.DB.Create(&user).Error; err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return &user, nil
}

// GetUserByID retrieves a user by ID.
func (s *Store) GetUserByID(id int64) (*model.User, error) {
	var user model.User
	if err := s.DB.First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}

// UpdateUserProfile updates nickname and avatar.
func (s *Store) UpdateUserProfile(id int64, nickname, avatarURL string) (*model.User, error) {
	updates := map[string]interface{}{}
	if nickname != "" {
		updates["nickname"] = nickname
	}
	if avatarURL != "" {
		updates["avatar_url"] = avatarURL
	}
	if len(updates) == 0 {
		return s.GetUserByID(id)
	}
	if err := s.DB.Model(&model.User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetUserByID(id)
}

// ===========================================================================
// Game operations
// ===========================================================================

// hashToken returns a SHA-256 hash of the invite token (PRD §7.3).
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// CreateGame creates a new forming game with the creator as first player.
func (s *Store) CreateGame(creatorID int64, name, inviteToken string) (*model.Game, error) {
	joinExpires := time.Now().Add(24 * time.Hour)
	game := model.Game{
		CreatorID:       creatorID,
		Name:            name,
		Status:          "forming",
		InviteTokenHash: hashToken(inviteToken),
		JoinExpiresAt:   &joinExpires,
		Version:         1,
	}
	if err := s.DB.Create(&game).Error; err != nil {
		return nil, fmt.Errorf("create game: %w", err)
	}

	// Add creator as first player.
	creator, _ := s.GetUserByID(creatorID)
	player := model.GamePlayer{
		GameID:           game.ID,
		UserID:           creatorID,
		NicknameSnapshot: creator.Nickname,
		Role:             "owner",
	}
	if err := s.DB.Create(&player).Error; err != nil {
		return nil, fmt.Errorf("add creator as player: %w", err)
	}

	return &game, nil
}

// GetGame retrieves a game by ID.
func (s *Store) GetGame(gameID int64) (*model.Game, error) {
	var game model.Game
	if err := s.DB.First(&game, gameID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNotFound
		}
		return nil, err
	}
	return &game, nil
}

// GetGameByInviteToken retrieves a game by its invite token hash.
func (s *Store) GetGameByInviteToken(token string) (*model.Game, error) {
	var game model.Game
	if err := s.DB.Where("invite_token_hash = ?", hashToken(token)).First(&game).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrInviteInvalid
		}
		return nil, err
	}
	return &game, nil
}

// GetActiveGames returns active and forming games for a user.
func (s *Store) GetActiveGames(userID int64) ([]model.Game, error) {
	var games []model.Game
	err := s.DB.Where("id IN (SELECT game_id FROM game_players WHERE user_id = ?) AND status IN ('forming', 'active')", userID).
		Order("updated_at DESC").Find(&games).Error
	return games, err
}

// GetHistoryGames returns ended/expired/cancelled games for a user with pagination.
func (s *Store) GetHistoryGames(userID int64, page, pageSize int) ([]model.Game, int64, error) {
	var games []model.Game
	var total int64

	baseQuery := s.DB.Where("id IN (SELECT game_id FROM game_players WHERE user_id = ?) AND status IN ('ended', 'expired', 'cancelled')", userID)
	if err := baseQuery.Model(&model.Game{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	if err := baseQuery.Order("updated_at DESC").Offset(offset).Limit(pageSize).Find(&games).Error; err != nil {
		return nil, 0, err
	}
	return games, total, nil
}

// GetGamePlayers returns all players in a game.
func (s *Store) GetGamePlayers(gameID int64) ([]model.GamePlayer, error) {
	var players []model.GamePlayer
	if err := s.DB.Where("game_id = ?", gameID).Order("joined_at ASC").Find(&players).Error; err != nil {
		return nil, err
	}
	return players, nil
}

// GetGamePlayer retrieves a specific game player.
func (s *Store) GetGamePlayer(gameID, userID int64) (*model.GamePlayer, error) {
	var player model.GamePlayer
	if err := s.DB.Where("game_id = ? AND user_id = ?", gameID, userID).First(&player).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrForbidden
		}
		return nil, err
	}
	return &player, nil
}

// CountGamePlayers returns the number of players in a game.
func (s *Store) CountGamePlayers(gameID int64) (int64, error) {
	var count int64
	err := s.DB.Model(&model.GamePlayer{}).Where("game_id = ?", gameID).Count(&count).Error
	return count, err
}

// JoinGame adds a user to a game as a player.
func (s *Store) JoinGame(gameID, userID int64, nickname string) (*model.GamePlayer, error) {
	// Check existing
	existing, err := s.GetGamePlayer(gameID, userID)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, errs.ErrForbidden) {
		return nil, err
	}

	// Check capacity
	count, err := s.CountGamePlayers(gameID)
	if err != nil {
		return nil, err
	}
	if count >= 4 {
		return nil, errs.ErrGameFull
	}

	player := model.GamePlayer{
		GameID:           gameID,
		UserID:           userID,
		NicknameSnapshot: nickname,
		Role:             "player",
	}
	if err := s.DB.Create(&player).Error; err != nil {
		// Race condition: unique constraint violation
		return nil, errs.ErrGameFull
	}
	return &player, nil
}

// UpdateGameStatus updates a game's status with optimistic locking.
func (s *Store) UpdateGameStatus(gameID int64, expectedStatus, newStatus string) (*model.Game, error) {
	var game model.Game
	if err := s.DB.First(&game, gameID).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	if game.Status != expectedStatus {
		return nil, errs.New("GAME_STATE_CHANGED", "牌桌状态已变更", errs.ActionRefreshGame)
	}
	game.Status = newStatus
	now := time.Now()
	switch newStatus {
	case "active":
		game.StartedAt = &now
	case "ended":
		game.EndedAt = &now
		game.SettlementUpdatedAt = &now
	case "expired":
		game.JoinExpiresAt = &now
		// Clear join expiry
	case "cancelled":
		game.JoinExpiresAt = &now
	}
	if err := s.DB.Save(&game).Error; err != nil {
		return nil, err
	}
	return &game, nil
}

// LockMembers marks a game's members as locked (PRD §2.1 rule 4).
func (s *Store) LockMembers(gameID int64) error {
	return s.DB.Model(&model.Game{}).Where("id = ?", gameID).
		Update("members_locked", true).Error
}

// InvalidateJoinExpiresAt clears the join expiry for a game.
func (s *Store) InvalidateJoinExpiresAt(gameID int64) error {
	return s.DB.Model(&model.Game{}).Where("id = ?", gameID).
		Update("join_expires_at", nil).Error
}

// UpdateSettlementTime updates the settlement_updated_at timestamp.
func (s *Store) UpdateSettlementTime(gameID int64) error {
	now := time.Now()
	return s.DB.Model(&model.Game{}).Where("id = ?", gameID).
		Update("settlement_updated_at", now).Error
}

// ===========================================================================
// Round operations
// ===========================================================================

// CreateRound creates a new round for a game with the given round number.
func (s *Store) CreateRound(gameID int64, roundNumber int) (*model.Round, error) {
	round := model.Round{
		GameID:      gameID,
		RoundNumber: roundNumber,
		Status:      "open",
	}
	if err := s.DB.Create(&round).Error; err != nil {
		// Unique constraint: round already exists (idempotent)
		var existing model.Round
		if err2 := s.DB.Where("game_id = ? AND round_number = ?", gameID, roundNumber).First(&existing).Error; err2 == nil {
			return &existing, nil
		}
		return nil, err
	}
	return &round, nil
}

// GetRound retrieves a round by ID.
func (s *Store) GetRound(roundID int64) (*model.Round, error) {
	var round model.Round
	if err := s.DB.First(&round, roundID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNotFound
		}
		return nil, err
	}
	return &round, nil
}

// GetCurrentRound retrieves the latest round for a game.
func (s *Store) GetCurrentRound(gameID int64) (*model.Round, error) {
	var round model.Round
	if err := s.DB.Where("game_id = ?", gameID).Order("round_number DESC").First(&round).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // no round yet
		}
		return nil, err
	}
	return &round, nil
}

// GetRoundsByGameID returns all rounds for a game, ordered by round number.
func (s *Store) GetRoundsByGameID(gameID int64) ([]model.Round, error) {
	var rounds []model.Round
	if err := s.DB.Where("game_id = ?", gameID).Order("round_number ASC").Find(&rounds).Error; err != nil {
		return nil, err
	}
	return rounds, nil
}

// UpdateRoundStatus updates a round's status.
func (s *Store) UpdateRoundStatus(roundID int64, expectedStatus, newStatus string) (*model.Round, error) {
	var round model.Round
	if err := s.DB.First(&round, roundID).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	if round.Status != expectedStatus {
		return nil, errs.New("ROUND_STATE_CHANGED", "局状态已变更", errs.ActionRefreshGame)
	}
	now := time.Now()
	updates := map[string]interface{}{"status": newStatus}
	switch newStatus {
	case "review":
		updates["review_started_at"] = now
	case "ready_for_next":
		updates["locked_at"] = now
	}
	if err := s.DB.Model(&model.Round{}).Where("id = ? AND status = ?", roundID, expectedStatus).
		Updates(updates).Error; err != nil {
		return nil, err
	}
	round.Status = newStatus
	return &round, nil
}

// CountLockedRounds returns the number of completed (ready_for_next) rounds.
func (s *Store) CountLockedRounds(gameID int64) (int, error) {
	var count int64
	err := s.DB.Model(&model.Round{}).Where("game_id = ? AND status = ?", gameID, "ready_for_next").
		Count(&count).Error
	return int(count), err
}

// ===========================================================================
// Submission operations
// ===========================================================================

// UpsertSubmission creates or updates a player's score submission for a round.
// PRD §2.3: each player can only submit their own score, lock before modifying.
func (s *Store) UpsertSubmission(roundID, gamePlayerID int64, score int, requestID string) (*model.RoundSubmission, error) {
	var sub model.RoundSubmission
	err := s.DB.Where("round_id = ? AND game_player_id = ?", roundID, gamePlayerID).First(&sub).Error
	if err == nil {
		// Update existing submission
		sub.Score = score
		sub.RequestID = requestID
		if err := s.DB.Save(&sub).Error; err != nil {
			return nil, err
		}
		return &sub, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	sub = model.RoundSubmission{
		RoundID:      roundID,
		GamePlayerID: gamePlayerID,
		Score:        score,
		RequestID:    requestID,
	}
	if err := s.DB.Create(&sub).Error; err != nil {
		return nil, err
	}
	return &sub, nil
}

// GetSubmissions returns all submissions for a round.
func (s *Store) GetSubmissions(roundID int64) ([]model.RoundSubmission, error) {
	var subs []model.RoundSubmission
	if err := s.DB.Where("round_id = ?", roundID).Find(&subs).Error; err != nil {
		return nil, err
	}
	return subs, nil
}

// GetSubmission retrieves a specific player's submission for a round.
func (s *Store) GetSubmission(roundID, gamePlayerID int64) (*model.RoundSubmission, error) {
	var sub model.RoundSubmission
	if err := s.DB.Where("round_id = ? AND game_player_id = ?", roundID, gamePlayerID).First(&sub).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // not submitted yet
		}
		return nil, err
	}
	return &sub, nil
}

// ===========================================================================
// Score adjustment operations (PRD §3.2)
// ===========================================================================

// CreateAdjustment creates a new pending score adjustment.
func (s *Store) CreateAdjustment(adj *model.ScoreAdjustment) error {
	if err := s.DB.Create(adj).Error; err != nil {
		// Idempotent: check if already exists
		var existing model.ScoreAdjustment
		if err2 := s.DB.Where("game_id = ? AND request_id = ?", adj.GameID, adj.RequestID).First(&existing).Error; err2 == nil {
			*adj = existing
			return nil
		}
		return err
	}
	return nil
}

// GetAdjustment retrieves an adjustment by ID.
func (s *Store) GetAdjustment(adjustmentID int64) (*model.ScoreAdjustment, error) {
	var adj model.ScoreAdjustment
	if err := s.DB.First(&adj, adjustmentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNotFound
		}
		return nil, err
	}
	return &adj, nil
}

// ResolveAdjustment transitions an adjustment to a resolved state (accepted/rejected/cancelled).
func (s *Store) ResolveAdjustment(adjustmentID int64, expectedStatus, newStatus string, resolvedBy int64) (*model.ScoreAdjustment, error) {
	var adj model.ScoreAdjustment
	if err := s.DB.First(&adj, adjustmentID).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	if adj.Status != expectedStatus {
		return &adj, errs.ErrAdjustResolved
	}
	// Check expiry
	if time.Now().After(adj.ExpiresAt) {
		return &adj, errs.ErrAdjustExpired
	}
	now := time.Now()
	updates := map[string]interface{}{
		"status":      newStatus,
		"resolved_by": resolvedBy,
		"resolved_at": now,
	}
	if err := s.DB.Model(&model.ScoreAdjustment{}).
		Where("id = ? AND status = ?", adjustmentID, expectedStatus).
		Updates(updates).Error; err != nil {
		return nil, err
	}
	adj.Status = newStatus
	adj.ResolvedBy = &resolvedBy
	adj.ResolvedAt = &now
	return &adj, nil
}

// GetAdjustmentsByGameID returns all adjustments for a game.
func (s *Store) GetAdjustmentsByGameID(gameID int64) ([]model.ScoreAdjustment, error) {
	var adjs []model.ScoreAdjustment
	if err := s.DB.Where("game_id = ?", gameID).Order("created_at DESC").Find(&adjs).Error; err != nil {
		return nil, err
	}
	return adjs, nil
}

// GetPendingAdjustmentsForUser returns pending adjustments where the user is the receiver.
func (s *Store) GetPendingAdjustmentsForUser(userID int64) ([]model.ScoreAdjustment, error) {
	var adjs []model.ScoreAdjustment
	err := s.DB.Where("to_player_id IN (SELECT id FROM game_players WHERE user_id = ?) AND status = ?", userID, "pending").
		Find(&adjs).Error
	return adjs, err
}

// GetAcceptedAdjustments returns all accepted adjustments for a game.
func (s *Store) GetAcceptedAdjustments(gameID int64) ([]model.ScoreAdjustment, error) {
	var adjs []model.ScoreAdjustment
	if err := s.DB.Where("game_id = ? AND status = ?", gameID, "accepted").Find(&adjs).Error; err != nil {
		return nil, err
	}
	return adjs, nil
}

// ===========================================================================
// Settlement aggregation (PRD §7.3)
// ===========================================================================

// PlayerTotal represents one player's final score and rank in a game.
type PlayerTotal struct {
	PlayerID    int64  `json:"player_id"`
	UserID      int64  `json:"user_id"`
	Nickname    string `json:"nickname"`
	TotalScore  int64  `json:"total_score"`
	Rank        int    `json:"rank"`
	Adjustments int    `json:"adjustments"`
}

// PlayerStanding represents one player's round-by-round score.
type RoundScore struct {
	RoundNumber int   `json:"round_number"`
	RoundID     int64 `json:"round_id"`
	Score       int   `json:"score"`
}

// AggregateSettlement computes final scores by aggregating locked round
// submissions plus accepted adjustments (PRD §7.3).
func (s *Store) AggregateSettlement(gameID int64) ([]PlayerTotal, int, error) {
	players, err := s.GetGamePlayers(gameID)
	if err != nil {
		return nil, 0, err
	}
	rounds, err := s.GetRoundsByGameID(gameID)
	if err != nil {
		return nil, 0, err
	}

	// Collect locked round IDs
	var lockedRoundIDs []int64
	for _, r := range rounds {
		if r.Status == "ready_for_next" || r.Status == "locked" {
			lockedRoundIDs = append(lockedRoundIDs, r.ID)
		}
	}

	// Get all submissions for locked rounds
	var allSubs []model.RoundSubmission
	if len(lockedRoundIDs) > 0 {
		if err := s.DB.Where("round_id IN ?", lockedRoundIDs).Find(&allSubs).Error; err != nil {
			return nil, 0, err
		}
	}

	// Sum scores per player
	totals := make(map[int64]int64) // game_player_id -> total
	for _, sub := range allSubs {
		totals[sub.GamePlayerID] += int64(sub.Score)
	}

	// Get accepted adjustments
	adjs, err := s.GetAcceptedAdjustments(gameID)
	if err != nil {
		return nil, 0, err
	}

	// Apply adjustments (PRD §7.3: from -amount, to +amount)
	adjCounts := make(map[int64]int)
	for _, adj := range adjs {
		totals[adj.FromPlayerID] -= int64(adj.Amount)
		totals[adj.ToPlayerID] += int64(adj.Amount)
		adjCounts[adj.FromPlayerID]++
		adjCounts[adj.ToPlayerID]++
	}

	// Build result
	result := make([]PlayerTotal, 0, len(players))
	for _, p := range players {
		result = append(result, PlayerTotal{
			PlayerID:    p.ID,
			UserID:      p.UserID,
			Nickname:    p.NicknameSnapshot,
			TotalScore:  totals[p.ID],
			Adjustments: adjCounts[p.ID],
		})
	}

	// Rank: sort by score descending, equal scores share rank
	sortPlayerTotals(result)

	totalAdjustments := len(adjs)
	return result, totalAdjustments, nil
}

// sortPlayerTotals sorts by TotalScore descending and assigns competition ranks.
func sortPlayerTotals(result []PlayerTotal) {
	// Insertion sort by score desc
	for i := 1; i < len(result); i++ {
		for j := i; j > 0 && result[j].TotalScore > result[j-1].TotalScore; j-- {
			result[j], result[j-1] = result[j-1], result[j]
		}
	}
	// Assign ranks
	for i := range result {
		if i == 0 || result[i].TotalScore != result[i-1].TotalScore {
			result[i].Rank = i + 1
		} else {
			result[i].Rank = result[i-1].Rank
		}
	}
}

// GetRoundScores returns the scores for each player in a specific round.
func (s *Store) GetRoundScores(roundID int64) ([]RoundScore, error) {
	var subs []model.RoundSubmission
	if err := s.DB.Where("round_id = ?", roundID).Find(&subs).Error; err != nil {
		return nil, err
	}
	result := make([]RoundScore, 0, len(subs))
	for _, sub := range subs {
		round, _ := s.GetRound(roundID)
		result = append(result, RoundScore{
			RoundNumber: round.RoundNumber,
			RoundID:     roundID,
			Score:       sub.Score,
		})
	}
	return result, nil
}

// ExpireOldFormingGames marks forming games older than 24h as expired.
func (s *Store) ExpireOldFormingGames() error {
	cutoff := time.Now().Add(-24 * time.Hour)
	return s.DB.Model(&model.Game{}).
		Where("status = ? AND created_at < ?", "forming", cutoff).
		Update("status", "expired").Error
}

// ExpireOldAdjustments marks pending adjustments past their expiry as expired.
func (s *Store) ExpireOldAdjustments() error {
	now := time.Now()
	return s.DB.Model(&model.ScoreAdjustment{}).
		Where("status = ? AND expires_at < ?", "pending", now).
		Update("status", "expired").Error
}

// Transaction wraps fn in a GORM transaction. If fn returns an error, the
// transaction is rolled back. This is used for critical write paths:
// lock round, end game, resolve adjustment (PRD §5.3).
func (s *Store) Transaction(fn func(tx *gorm.DB) error) error {
	return s.DB.Transaction(fn)
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	sqlDB, err := s.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
