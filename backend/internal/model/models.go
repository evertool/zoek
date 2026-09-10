package model

import (
	"time"

	"gorm.io/gorm"
)

// User maps to the users table (PRD §7.2).
type User struct {
	ID       int64  `gorm:"primaryKey;autoIncrement" json:"id"`
	OpenID   string `gorm:"column:openid;type:varchar(64);uniqueIndex;not null" json:"openid"`
	Nickname string `gorm:"type:varchar(32);not null" json:"nickname"`
	// AvatarURL 保存头像的相对路径（如 /uploads/avatars/xxx.jpg）
	AvatarURL string `gorm:"type:varchar(256)" json:"avatar_url"`
	// ProfileCompleted 标记用户是否已完善资料（有昵称+持久化头像）。
	// 登录时直接读此字段，避免每次重新推导；保存资料时置 true。
	ProfileCompleted bool `gorm:"not null;default:false" json:"profile_completed"`
	// 排位数据（4 人局散台时结算），段位由 RankStars 推导，见 internal/rank。
	RankStars      int `gorm:"not null;default:0" json:"rank_stars"`
	RankWins       int `gorm:"not null;default:0" json:"rank_wins"`
	RankDraws      int `gorm:"not null;default:0" json:"rank_draws"`
	RankLosses     int `gorm:"not null;default:0" json:"rank_losses"`
	RankStreak     int `gorm:"not null;default:0" json:"rank_streak"`
	RankBestStreak int `gorm:"not null;default:0" json:"rank_best_streak"`
	RankPoints     int `gorm:"not null;default:0" json:"rank_points"` // 赛季净胜分
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

func (User) TableName() string { return "users" }

// RankSettlement 记录一场排位局（4 人局）散台后每位玩家的星级变动，
// 用于结算页展示「本场 +N 星」以及防止重复结算（uk_rank_game_user）。
type RankSettlement struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	GameID      int64     `gorm:"not null;uniqueIndex:uk_rank_game_user" json:"game_id"`
	UserID      int64     `gorm:"not null;uniqueIndex:uk_rank_game_user" json:"user_id"`
	Result      string    `gorm:"type:varchar(8);not null" json:"result"` // win/draw/lose
	Score       int       `gorm:"not null" json:"score"`
	StarsDelta  int       `gorm:"not null" json:"stars_delta"` // 含连胜奖励与保底钳制后的实际变动
	BonusStars  int       `gorm:"not null;default:0" json:"bonus_stars"`
	StreakAfter int       `gorm:"not null;default:0" json:"streak_after"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (RankSettlement) TableName() string { return "rank_settlements" }

// Game maps to the games table (PRD §7.2).
type Game struct {
	ID                  int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatorID           int64          `gorm:"not null" json:"creator_id"`
	Name                string         `gorm:"type:varchar(64);not null;default:未命名牌局" json:"name"`
	Status              string         `gorm:"type:varchar(16);not null;default:forming" json:"status"`
	InviteTokenHash     string         `gorm:"type:varchar(128);uniqueIndex;not null" json:"-"`
	JoinExpiresAt       *time.Time     `json:"join_expires_at,omitempty"`
	MembersLocked       bool           `gorm:"not null;default:false" json:"members_locked"`
	StartedAt           *time.Time     `json:"started_at,omitempty"`
	EndedAt             *time.Time     `json:"ended_at,omitempty"`
	SettlementUpdatedAt *time.Time     `json:"settlement_updated_at,omitempty"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	DeletedAt           gorm.DeletedAt `gorm:"index" json:"-"`
	Version             int            `gorm:"not null;default:1" json:"version"`
}

func (Game) TableName() string { return "games" }

// GamePlayer maps to the game_players table (PRD §7.2).
type GamePlayer struct {
	ID               int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	GameID           int64     `gorm:"not null;uniqueIndex:uk_game_user" json:"game_id"`
	UserID           int64     `gorm:"not null;uniqueIndex:uk_game_user" json:"user_id"`
	NicknameSnapshot string    `gorm:"type:varchar(32);not null" json:"nickname_snapshot"`
	Role             string    `gorm:"type:varchar(16);not null;default:player" json:"role"`
	Seat             int       `gorm:"not null;default:0" json:"seat"` // 座位号 1-4（東南西北），0=旧数据未分配
	JoinedAt         time.Time `gorm:"autoCreateTime" json:"joined_at"`
}

func (GamePlayer) TableName() string { return "game_players" }

// Round maps to the rounds table (PRD §7.2).
type Round struct {
	ID              int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	GameID          int64      `gorm:"not null;uniqueIndex:uk_game_round" json:"game_id"`
	RoundNumber     int        `gorm:"not null;uniqueIndex:uk_game_round" json:"round_number"`
	Status          string     `gorm:"type:varchar(16);not null;default:open" json:"status"`
	ReviewStartedAt *time.Time `json:"review_started_at,omitempty"`
	LockedAt        *time.Time `json:"locked_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

func (Round) TableName() string { return "rounds" }

// RoundSubmission maps to the round_submissions table (PRD §7.2).
type RoundSubmission struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	RoundID      int64     `gorm:"not null;uniqueIndex:uk_round_player" json:"round_id"`
	GamePlayerID int64     `gorm:"not null;uniqueIndex:uk_round_player" json:"game_player_id"`
	Score        int       `gorm:"not null" json:"score"`
	RequestID    string    `gorm:"type:varchar(64);not null;uniqueIndex:uk_round_request" json:"request_id"`
	SubmittedAt  time.Time `json:"submitted_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (RoundSubmission) TableName() string { return "round_submissions" }

// ScoreAdjustment maps to the score_adjustments table (PRD §7.2).
type ScoreAdjustment struct {
	ID             int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	GameID         int64      `gorm:"not null;uniqueIndex:uk_adjustment_request" json:"game_id"`
	RoundID        int64      `gorm:"not null" json:"round_id"`
	FromPlayerID   int64      `gorm:"not null" json:"from_player_id"`
	ToPlayerID     int64      `gorm:"not null" json:"to_player_id"`
	AdjustmentType string     `gorm:"type:varchar(16);not null" json:"adjustment_type"`
	Amount         int        `gorm:"not null" json:"amount"`
	Reason         string     `gorm:"type:varchar(256)" json:"reason,omitempty"`
	ProposedBy     int64      `gorm:"not null" json:"proposed_by"`
	Status         string     `gorm:"type:varchar(16);not null;default:pending" json:"status"`
	RequestID      string     `gorm:"type:varchar(64);not null;uniqueIndex:uk_adjustment_request" json:"request_id"`
	ExpiresAt      time.Time  `gorm:"not null" json:"expires_at"`
	ResolvedBy     *int64     `json:"resolved_by,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
}

func (ScoreAdjustment) TableName() string { return "score_adjustments" }

// GameHidden records a user hiding an ended game from their own history list
// (用户删除对局记录：仅对自己隐藏，不影响其他参与者)。
type GameHidden struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	GameID    int64     `gorm:"not null;uniqueIndex:uk_game_hidden" json:"game_id"`
	UserID    int64     `gorm:"not null;uniqueIndex:uk_game_hidden" json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (GameHidden) TableName() string { return "game_hiddens" }

// SeatSwapRequest 记录一次「两个已入座玩家互换座位」的申请（PRD §8.7）。
// 长按空位是即时换座（走 swap_seat），长按他人座位才需要对方确认，落在本表。
type SeatSwapRequest struct {
	ID           int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	GameID       int64      `gorm:"not null;index:idx_swap_game" json:"game_id"`
	FromPlayerID int64      `gorm:"not null" json:"from_player_id"`
	ToPlayerID   int64      `gorm:"not null;index:idx_swap_to" json:"to_player_id"`
	FromSeat     int        `gorm:"not null" json:"from_seat"`
	ToSeat       int        `gorm:"not null" json:"to_seat"`
	Status       string     `gorm:"type:varchar(16);not null;default:pending" json:"status"` // pending/accepted/rejected/cancelled/expired
	ExpiresAt    time.Time  `gorm:"not null" json:"expires_at"`
	CreatedAt    time.Time  `json:"created_at"`
	ResolvedAt   *time.Time `json:"resolved_at,omitempty"`
}

func (SeatSwapRequest) TableName() string { return "seat_swap_requests" }
