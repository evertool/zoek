package store

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/lk/zoek/backend/internal/errs"
	"github.com/lk/zoek/backend/internal/logger"
	"github.com/lk/zoek/backend/internal/model"
	"github.com/lk/zoek/backend/internal/rank"
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
	if err := s.DB.AutoMigrate(
		&model.User{},
		&model.Game{},
		&model.GamePlayer{},
		&model.Round{},
		&model.RoundSubmission{},
		&model.ScoreAdjustment{},
		&model.GameHidden{},
		&model.RankSettlement{},
	); err != nil {
		return err
	}
	// seat 字段后加，旧数据按加入顺序回填座位号
	return s.backfillPlayerSeats()
}

// backfillPlayerSeats 为 seat=0 的旧数据按加入顺序分配 1..4 中未被占用的最小编号。
func (s *Store) backfillPlayerSeats() error {
	var gameIDs []int64
	if err := s.DB.Model(&model.GamePlayer{}).Where("seat = 0").Distinct().Pluck("game_id", &gameIDs).Error; err != nil {
		return err
	}
	for _, gameID := range gameIDs {
		var players []model.GamePlayer
		if err := s.DB.Where("game_id = ?", gameID).Order("joined_at ASC, id ASC").Find(&players).Error; err != nil {
			return err
		}
		used := map[int]bool{}
		for _, p := range players {
			if p.Seat >= 1 && p.Seat <= 4 {
				used[p.Seat] = true
			}
		}
		for _, p := range players {
			if p.Seat >= 1 && p.Seat <= 4 {
				continue
			}
			for seat := 1; seat <= 4; seat++ {
				if used[seat] {
					continue
				}
				used[seat] = true
				if err := s.DB.Model(&model.GamePlayer{}).Where("id = ?", p.ID).Update("seat", seat).Error; err != nil {
					return err
				}
				break
			}
		}
	}
	return nil
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
// avatarURL 应为服务器相对路径（如 /uploads/avatars/xxx.jpg）。
// 昵称+头像都有效时，同时将 ProfileCompleted 置 true。
func (s *Store) UpdateUserProfile(id int64, nickname, avatarURL string) (*model.User, error) {
	updates := map[string]interface{}{}
	if nickname != "" {
		updates["nickname"] = nickname
	}
	if avatarURL != "" {
		updates["avatar_url"] = avatarURL
	}
	// 昵称+头像都有效 → 标记资料已完善
	if nickname != "" && avatarURL != "" {
		updates["profile_completed"] = true
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
		Seat:             1,
		JoinedAt:         time.Now(),
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

// GetUserActiveGameID 返回用户当前进行中（forming/active）的牌局 ID，excludeGameID 用于
// 「加入该局本身不算冲突」的场景；没有进行中的牌局返回 0。
func (s *Store) GetUserActiveGameID(userID, excludeGameID int64) (int64, error) {
	query := s.DB.Model(&model.Game{}).Where(
		"status IN ('forming', 'active') AND id IN (SELECT game_id FROM game_players WHERE user_id = ?)", userID)
	if excludeGameID > 0 {
		query = query.Where("id <> ?", excludeGameID)
	}
	var ids []int64
	if err := query.Order("updated_at DESC").Limit(1).Pluck("id", &ids).Error; err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	return ids[0], nil
}

// GetHistoryGames returns ended/expired/cancelled games for a user with
// pagination, excluding games the user has hidden from their history.
// HistoryGameFilters 记录页筛选条件。
type HistoryGameFilters struct {
	Days int // 最近 N 天，0 = 全部
}

// GetHistoryGamesAll 返回用户全部历史牌局（按结束时间倒序）。
// 标签（胜/平/负）筛选需逐局比较得分，由 handler 配合 GetGamePlayerTotals 完成后分页。
func (s *Store) GetHistoryGamesAll(userID int64, f HistoryGameFilters) ([]model.Game, error) {
	query := s.DB.Model(&model.Game{}).Where(
		"id IN (SELECT game_id FROM game_players WHERE user_id = ?) AND status IN ('ended', 'expired', 'cancelled') "+
			"AND id NOT IN (SELECT game_id FROM game_hiddens WHERE user_id = ?)", userID, userID)
	if f.Days > 0 {
		cutoff := time.Now().AddDate(0, 0, -f.Days)
		query = query.Where("COALESCE(ended_at, created_at) >= ?", cutoff)
	}
	var games []model.Game
	if err := query.Order("COALESCE(ended_at, created_at) DESC").Find(&games).Error; err != nil {
		return nil, err
	}
	return games, nil
}

// GetGamePlayers returns all players in a game.
func (s *Store) GetGamePlayers(gameID int64) ([]model.GamePlayer, error) {
	var players []model.GamePlayer
	if err := s.DB.Where("game_id = ?", gameID).Order("seat ASC, joined_at ASC").Find(&players).Error; err != nil {
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

// GamePlayerTotal 一位玩家在一场牌局里的总得分（仅统计已锁定局）。
type GamePlayerTotal struct {
	GamePlayerID int64
	Total        int
}

// GetGamePlayerTotals 汇总一场牌局各玩家的锁定局总得分。
func (s *Store) GetGamePlayerTotals(gameID int64) (map[int64]int, error) {
	var rows []GamePlayerTotal
	err := s.DB.Table("round_submissions rs").
		Select("rs.game_player_id AS game_player_id, COALESCE(SUM(rs.score), 0) AS total").
		Joins("JOIN rounds r ON r.id = rs.round_id").
		Where("r.game_id = ? AND r.status IN ('ready_for_next', 'locked')", gameID).
		Group("rs.game_player_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	totals := make(map[int64]int, len(rows))
	for _, row := range rows {
		totals[row.GamePlayerID] = row.Total
	}
	return totals, nil
}

// SettleGameRank 散台后结算排位星级，仅 4 人局参与；重复调用幂等跳过。
// 在 EndGame 事务外调用（EndGame 先行落库），结算自身用事务保证原子性。
func (s *Store) SettleGameRank(gameID int64) error {
	return s.Transaction(func(tx *gorm.DB) error {
		// 幂等：此局已结算过
		var settled int64
		if err := tx.Model(&model.RankSettlement{}).Where("game_id = ?", gameID).Count(&settled).Error; err != nil {
			return err
		}
		if settled > 0 {
			return nil
		}

		var players []model.GamePlayer
		if err := tx.Where("game_id = ?", gameID).Order("seat ASC").Find(&players).Error; err != nil {
			return err
		}
		if len(players) != 4 {
			return nil // 排位规定：只有 4 人局参与排位
		}

		gpIDs := make([]int64, 0, len(players))
		playerByGP := make(map[int64]*model.GamePlayer, len(players))
		for i := range players {
			gpIDs = append(gpIDs, players[i].ID)
			playerByGP[players[i].ID] = &players[i]
		}

		var rows []GamePlayerTotal
		if err := tx.Table("round_submissions rs").
			Select("rs.game_player_id AS game_player_id, COALESCE(SUM(rs.score), 0) AS total").
			Joins("JOIN rounds r ON r.id = rs.round_id").
			Where("rs.game_player_id IN ? AND r.status IN ('ready_for_next', 'locked')", gpIDs).
			Group("rs.game_player_id").
			Scan(&rows).Error; err != nil {
			return err
		}
		scores := make(map[int64]int, len(players))
		for _, gp := range players {
			scores[gp.ID] = 0
		}
		for _, row := range rows {
			scores[row.GamePlayerID] = row.Total
		}

		// 取玩家当前排位数据
		userIDs := make([]int64, 0, len(players))
		for _, gp := range players {
			userIDs = append(userIDs, gp.UserID)
		}
		var users []model.User
		if err := tx.Where("id IN ?", userIDs).Find(&users).Error; err != nil {
			return err
		}
		userByID := make(map[int64]*model.User, len(users))
		for i := range users {
			userByID[users[i].ID] = &users[i]
		}

		streaks := make(map[int64]int, len(players))
		for _, gp := range players {
			streaks[gp.ID] = userByID[gp.UserID].RankStreak
		}

		outcomes := rank.Settle(scores, streaks)
		for gpID, o := range outcomes {
			gp := playerByGP[gpID]
			u := userByID[gp.UserID]

			newStars := u.RankStars + o.StarsDelta
			if newStars < 0 {
				newStars = 0 // 九品保底：0 星不再扣
			}
			if err := tx.Model(&model.User{}).Where("id = ?", u.ID).Updates(map[string]interface{}{
				"rank_stars":       newStars,
				"rank_wins":        u.RankWins + boolInt(o.Result == "win"),
				"rank_draws":       u.RankDraws + boolInt(o.Result == "draw"),
				"rank_losses":      u.RankLosses + boolInt(o.Result == "lose"),
				"rank_streak":      o.StreakAfter,
				"rank_best_streak": maxInt(u.RankBestStreak, o.StreakAfter),
				"rank_points":      u.RankPoints + scores[gpID],
			}).Error; err != nil {
				return err
			}
			if err := tx.Create(&model.RankSettlement{
				GameID:      gameID,
				UserID:      u.ID,
				Result:      o.Result,
				Score:       scores[gpID],
				StarsDelta:  newStars - u.RankStars, // 钳制后的实际变动
				BonusStars:  o.BonusStars,
				StreakAfter: o.StreakAfter,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// GetRankSettlements 一场牌局的排位结算记录。
func (s *Store) GetRankSettlements(gameID int64) ([]model.RankSettlement, error) {
	var rows []model.RankSettlement
	err := s.DB.Where("game_id = ?", gameID).Find(&rows).Error
	return rows, err
}

// CountLockedRoundsByGameIDs 批量统计各局已锁定局数。
func (s *Store) CountLockedRoundsByGameIDs(gameIDs []int64) (map[int64]int64, error) {
	var rows []struct {
		GameID int64
		Cnt    int64
	}
	err := s.DB.Model(&model.Round{}).
		Select("game_id, COUNT(*) AS cnt").
		Where("game_id IN ? AND status IN ('ready_for_next', 'locked')", gameIDs).
		Group("game_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[int64]int64, len(rows))
	for _, row := range rows {
		out[row.GameID] = row.Cnt
	}
	return out, nil
}

// GetPlayersByGameIDs 批量取多局玩家。
func (s *Store) GetPlayersByGameIDs(gameIDs []int64) ([]model.GamePlayer, error) {
	var players []model.GamePlayer
	err := s.DB.Where("game_id IN ?", gameIDs).Order("seat ASC, joined_at ASC").Find(&players).Error
	return players, err
}

// GetTotalsByGameIDs 批量取多局的各玩家锁定局总得分：game_id -> game_player_id -> total。
func (s *Store) GetTotalsByGameIDs(gameIDs []int64) (map[int64]map[int64]int, error) {
	var rows []struct {
		GameID       int64
		GamePlayerID int64
		Total        int
	}
	err := s.DB.Table("round_submissions rs").
		Select("r.game_id AS game_id, rs.game_player_id AS game_player_id, COALESCE(SUM(rs.score), 0) AS total").
		Joins("JOIN rounds r ON r.id = rs.round_id").
		Where("r.game_id IN ? AND r.status IN ('ready_for_next', 'locked')", gameIDs).
		Group("r.game_id, rs.game_player_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[int64]map[int64]int, len(gameIDs))
	for _, row := range rows {
		if out[row.GameID] == nil {
			out[row.GameID] = make(map[int64]int)
		}
		out[row.GameID][row.GamePlayerID] = row.Total
	}
	return out, nil
}

// CountAdjustmentsByGameIDs 批量统计各局的改分记录数。
func (s *Store) CountAdjustmentsByGameIDs(gameIDs []int64) (map[int64]int64, error) {
	var rows []struct {
		GameID int64
		Cnt    int64
	}
	err := s.DB.Model(&model.ScoreAdjustment{}).
		Select("game_id, COUNT(*) AS cnt").
		Where("game_id IN ?", gameIDs).
		Group("game_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[int64]int64, len(rows))
	for _, row := range rows {
		out[row.GameID] = row.Cnt
	}
	return out, nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
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

	seat, err := s.freeSeat(gameID)
	if err != nil {
		return nil, err
	}
	player := model.GamePlayer{
		GameID:           gameID,
		UserID:           userID,
		NicknameSnapshot: nickname,
		Role:             "player",
		Seat:             seat,
		JoinedAt:         time.Now(),
	}
	if err := s.DB.Create(&player).Error; err != nil {
		// Race condition: unique constraint violation
		return nil, errs.ErrGameFull
	}
	return &player, nil
}

// freeSeat returns the lowest unoccupied seat number (1-4) in the game.
func (s *Store) freeSeat(gameID int64) (int, error) {
	var taken []int
	if err := s.DB.Model(&model.GamePlayer{}).Where("game_id = ?", gameID).Pluck("seat", &taken).Error; err != nil {
		return 0, err
	}
	used := make(map[int]bool, len(taken))
	for _, seat := range taken {
		used[seat] = true
	}
	for seat := 1; seat <= 4; seat++ {
		if !used[seat] {
			return seat, nil
		}
	}
	return 0, errs.ErrGameFull
}

// SwapToEmptySeat moves a player to an unoccupied seat immediately.
// Occupied seats need the other player's consent and are rejected here.
func (s *Store) SwapToEmptySeat(gameID, userID int64, targetSeat int) (*model.GamePlayer, error) {
	if targetSeat < 1 || targetSeat > 4 {
		return nil, errs.ErrInvalidInput
	}
	player, err := s.GetGamePlayer(gameID, userID)
	if err != nil {
		return nil, err
	}
	if player.Seat == targetSeat {
		return player, nil
	}
	var count int64
	if err := s.DB.Model(&model.GamePlayer{}).Where("game_id = ? AND seat = ?", gameID, targetSeat).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errs.ErrSeatOccupied
	}
	if err := s.DB.Model(&model.GamePlayer{}).Where("id = ?", player.ID).Update("seat", targetSeat).Error; err != nil {
		return nil, err
	}
	return s.GetGamePlayer(gameID, userID)
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

// StartGameIfReady atomically activates a forming game once it has at least 2
// players and creates round 1. Concurrent joins race safely: only the request
// whose conditional update touches the row creates the round.
func (s *Store) StartGameIfReady(gameID int64) (bool, error) {
	started := false
	err := s.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&model.GamePlayer{}).Where("game_id = ?", gameID).Count(&count).Error; err != nil {
			return err
		}
		if count < 2 {
			return nil
		}
		now := time.Now()
		res := tx.Model(&model.Game{}).
			Where("id = ? AND status = ?", gameID, "forming").
			Updates(map[string]interface{}{"status": "active", "started_at": now})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil // already started by a concurrent request
		}
		round := model.Round{GameID: gameID, RoundNumber: 1, Status: "open"}
		if err := tx.Create(&round).Error; err != nil {
			return err
		}
		started = true
		return nil
	})
	return started, err
}

// HideGame hides an ended/expired/cancelled game from the user's history list.
// Idempotent: duplicates are treated as success.
func (s *Store) HideGame(userID, gameID int64) error {
	var n int64
	if err := s.DB.Model(&model.GameHidden{}).
		Where("game_id = ? AND user_id = ?", gameID, userID).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	return s.DB.Create(&model.GameHidden{GameID: gameID, UserID: userID}).Error
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
	AvatarURL   string `json:"avatar_url"`
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
		var avatarURL string
		if u, err := s.GetUserByID(p.UserID); err == nil {
			avatarURL = u.AvatarURL
		}
		result = append(result, PlayerTotal{
			PlayerID:    p.ID,
			UserID:      p.UserID,
			Nickname:    p.NicknameSnapshot,
			AvatarURL:   avatarURL,
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

// ===========================================================================
// Leaderboard and personal stats (PRD §1.4 P1 雀友榜)
// ===========================================================================

// LeaderboardEntry is one user's aggregate performance inside the viewer's
// 雀友圈（同过台的玩家）.
type LeaderboardEntry struct {
	UserID      int64    `json:"user_id"`
	Nickname    string   `json:"nickname"`
	AvatarURL   string   `json:"avatar_url"`
	Games       int      `json:"games"`
	Wins        int      `json:"wins"`
	Top3        int      `json:"top3"`
	TotalScore  int64    `json:"total_score"` // 窗口内净胜分
	WinRate     float64  `json:"win_rate"`
	Top3Rate    float64  `json:"top3_rate"`
	AvgRank     float64  `json:"avg_rank"`
	BestStreak  int      `json:"best_streak"` // 窗口内最高连胜（连续第1名）
	BestScore   int      `json:"best_score"`  // 窗口内单场最高分
	Tags        []string `json:"tags"`        // 规则标签：连胜王/今晚手气王/稳如泰山/大翻盘赢家/常客/铁脚/雀神
	IsSelf      bool     `json:"is_self"`
	Qualified   bool     `json:"qualified"` // 完成局数达到门槛，进入正式榜单
	TierName    string   `json:"tier_name"` // 排位段位全名
	TierShort   string   `json:"tier_short"`
	Grade       string   `json:"grade"`
	Stars       int      `json:"stars"` // 段内星级
	RankStars   int      `json:"rank_stars"` // 排位累计星（排位榜排序用）
}

// TrendPoint is one ended game's final score for the personal trend chart.
type TrendPoint struct {
	GameID  int64  `json:"game_id"`
	Name    string `json:"name"`
	Total   int64  `json:"total"`
	EndedAt string `json:"ended_at"`
}

// UserStats is the personal performance summary with per-game trend.
type UserStats struct {
	Games     int          `json:"games"`
	Wins      int          `json:"wins"`
	Top3      int          `json:"top3"`
	WinRate   float64      `json:"win_rate"`
	Top3Rate  float64      `json:"top3_rate"`
	AvgRank   float64      `json:"avg_rank"`
	BestScore int64        `json:"best_score"`
	Trend     []TrendPoint `json:"trend"`
}

// GetLeaderboard aggregates ended games in the last `days` days across the
// viewer's co-play games（days<=0 表示不限时间）。同台切磋满 minGames 场才可入榜，
// 最多返回 20 人。标签规则：
//   - 连胜王：窗口内最高连胜 ≥ 3；       - 雀神：≥30 场且胜率 ≥ 60%；
//   - 常客：≥10 场；                     - 铁脚：≥50 场；
//   - 稳如泰山：≥5 场且场均 |得分| ≤ 10；
//   - 今晚手气王：今日单场最高分 ≥ 20；
//   - 大翻盘赢家：单场从最深落后翻回（终局-最低点 ≥ 30 且终局为正）。
func (s *Store) GetLeaderboard(userID int64, days, minGames int) ([]LeaderboardEntry, error) {
	query := s.DB.Where(
		"status = 'ended' AND ended_at IS NOT NULL AND id IN "+
			"(SELECT game_id FROM game_players WHERE user_id = ?)", userID)
	if days > 0 {
		query = query.Where("ended_at >= ?", time.Now().AddDate(0, 0, -days))
	}
	var games []model.Game
	if err := query.Order("ended_at ASC").Find(&games).Error; err != nil {
		return nil, err
	}

	type acc struct {
		games, wins, top3, rankSum int
		netSum                     int64
		bestStreak, curStreak      int
		bestScore, todayBest       int
		sumAbs                     int64
		comeback                   bool
		nick, avatar               string
	}
	accs := map[int64]*acc{}
	nick := map[int64]string{}
	gpUser := map[int64]int64{} // game_player_id -> user_id（逐场更新）
	today := time.Now().Format("2006-01-02")

	for _, g := range games {
		totals, _, err := s.AggregateSettlement(g.ID)
		if err != nil {
			continue
		}
		for _, pt := range totals {
			a := accs[pt.UserID]
			if a == nil {
				a = &acc{}
				accs[pt.UserID] = a
			}
			a.games++
			a.rankSum += pt.Rank
			a.netSum += pt.TotalScore
			if pt.Rank == 1 {
				a.wins++
				a.curStreak++
				if a.curStreak > a.bestStreak {
					a.bestStreak = a.curStreak
				}
			} else {
				a.curStreak = 0
			}
			if int(pt.TotalScore) > a.bestScore {
				a.bestScore = int(pt.TotalScore)
			}
			a.sumAbs += absI64(pt.TotalScore)
			if g.EndedAt != nil && g.EndedAt.Format("2006-01-02") == today && int(pt.TotalScore) > a.todayBest {
				a.todayBest = int(pt.TotalScore)
			}
			nick[pt.UserID] = pt.Nickname
			gpUser[pt.PlayerID] = pt.UserID
		}

		// 大翻盘：单场逐局累计，最深落后翻回 ≥ 30 分且终局为正
		cums, err := s.GetGameRoundScores(g.ID)
		if err == nil {
			for gpID, seq := range cums {
				uid, ok := gpUser[gpID]
				if !ok || len(seq) == 0 {
					continue
				}
				minV, finalV := seq[0], seq[len(seq)-1]
				for _, v := range seq {
					if v < minV {
						minV = v
					}
				}
				if minV < 0 && finalV > 0 && finalV-minV >= 30 {
					accs[uid].comeback = true
				}
			}
		}
	}

	entries := make([]LeaderboardEntry, 0, len(accs))
	for uid, a := range accs {
		if a.games < minGames {
			continue // 同台切磋 ≥ minGames 场才可入榜
		}
		if a.nick == "" {
			if u, err := s.GetUserByID(uid); err == nil {
				a.nick = u.Nickname
			}
		}
		var avatarURL string
		if u, err := s.GetUserByID(uid); err == nil {
			avatarURL = u.AvatarURL
		}
		e := LeaderboardEntry{
			UserID:    uid,
			Nickname:  a.nick,
			AvatarURL: avatarURL,
			Games:     a.games,
			Wins:      a.wins,
			Top3:      a.top3,
			IsSelf:    uid == userID,
			Qualified: true,
		}
		if a.games > 0 {
			e.WinRate = round2(float64(a.wins) / float64(a.games) * 100)
			e.Top3Rate = round2(float64(a.top3) / float64(a.games) * 100)
			e.AvgRank = round2(float64(a.rankSum) / float64(a.games))
		}
		e.BestStreak = a.bestStreak
		e.BestScore = a.bestScore
		e.TotalScore = a.netSum

		// 标签规则
		tags := []string{}
		if a.bestStreak >= 3 {
			tags = append(tags, "连胜王")
		}
		if a.todayBest >= 20 {
			tags = append(tags, "今晚手气王")
		}
		if a.games >= 5 && float64(a.sumAbs)/float64(a.games) <= 10 {
			tags = append(tags, "稳如泰山")
		}
		if a.comeback {
			tags = append(tags, "大翻盘赢家")
		}
		if a.games >= 50 {
			tags = append(tags, "铁脚")
		} else if a.games >= 10 {
			tags = append(tags, "常客")
		}
		if a.games >= 30 && e.WinRate >= 60 {
			tags = append(tags, "雀神")
		}
		e.Tags = tags

		if u, err := s.GetUserByID(uid); err == nil {
			info := rank.InfoFromStars(u.RankStars)
			e.TierName = info.TierName
			e.TierShort = info.TierShort
			e.Grade = info.Grade
			e.Stars = info.StarsInTier
			e.RankStars = u.RankStars
		}
		entries = append(entries, e)
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].WinRate != entries[j].WinRate {
			return entries[i].WinRate > entries[j].WinRate
		}
		if entries[i].AvgRank != entries[j].AvgRank {
			return entries[i].AvgRank < entries[j].AvgRank
		}
		return entries[i].Games > entries[j].Games
	})
	if len(entries) > 20 {
		entries = entries[:20] // 只显示前 20
	}
	return entries, nil
}

// GetGameRoundScores 一场牌局中各玩家按已锁定局顺序的累计得分序列。
func (s *Store) GetGameRoundScores(gameID int64) (map[int64][]int64, error) {
	var rows []struct {
		GamePlayerID int64
		Score        int
	}
	err := s.DB.Table("round_submissions rs").
		Select("rs.game_player_id AS game_player_id, rs.score AS score").
		Joins("JOIN rounds r ON r.id = rs.round_id").
		Where("r.game_id = ? AND r.status IN ('ready_for_next', 'locked')", gameID).
		Order("r.round_number ASC, rs.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	seq := map[int64][]int64{}
	cum := map[int64]int64{}
	for _, row := range rows {
		cum[row.GamePlayerID] += int64(row.Score)
		seq[row.GamePlayerID] = append(seq[row.GamePlayerID], cum[row.GamePlayerID])
	}
	return seq, nil
}

func absI64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// GetUserStats summarizes the user's ended games (all time) with a trend of
// the last maxTrend games' final scores (chronological).
func (s *Store) GetUserStats(userID int64, maxTrend int) (*UserStats, error) {
	var games []model.Game
	err := s.DB.Where(
		"status = 'ended' AND ended_at IS NOT NULL AND id IN "+
			"(SELECT game_id FROM game_players WHERE user_id = ?)", userID).
		Order("ended_at ASC").Find(&games).Error
	if err != nil {
		return nil, err
	}

	st := &UserStats{Trend: []TrendPoint{}}
	rankSum := 0
	for _, g := range games {
		totals, _, err := s.AggregateSettlement(g.ID)
		if err != nil {
			continue
		}
		var mine *PlayerTotal
		for i := range totals {
			if totals[i].UserID == userID {
				mine = &totals[i]
			}
		}
		if mine == nil {
			continue
		}
		st.Games++
		rankSum += mine.Rank
		if mine.Rank == 1 {
			st.Wins++
		}
		if mine.Rank <= 3 {
			st.Top3++
		}
		if mine.TotalScore > st.BestScore {
			st.BestScore = mine.TotalScore
		}
		st.Trend = append(st.Trend, TrendPoint{
			GameID:  g.ID,
			Name:    g.Name,
			Total:   mine.TotalScore,
			EndedAt: g.EndedAt.Format("01-02"),
		})
	}
	if len(st.Trend) > maxTrend {
		st.Trend = st.Trend[len(st.Trend)-maxTrend:]
	}
	if st.Games > 0 {
		st.WinRate = round2(float64(st.Wins) / float64(st.Games) * 100)
		st.Top3Rate = round2(float64(st.Top3) / float64(st.Games) * 100)
		st.AvgRank = round2(float64(rankSum) / float64(st.Games))
	}
	return st, nil
}

// round2 rounds a float to 2 decimal places.
func round2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
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
