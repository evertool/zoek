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
	// AvatarURL 保存 base64 数据 URL（MVP 无对象存储，头像经压缩后持久存库）
	AvatarURL string         `gorm:"type:mediumtext" json:"avatar_url"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (User) TableName() string { return "users" }

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
