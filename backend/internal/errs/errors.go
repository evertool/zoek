package errs

import "fmt"

// Action tells the frontend what to do next (PRD §5.1).
type Action string

const (
	ActionReLogin     Action = "RELOGIN"
	ActionRefreshGame Action = "REFRESH_GAME"
	ActionRetry       Action = "RETRY"
	ActionBackToRoom  Action = "BACK_TO_ROOM"
	ActionReadOnly    Action = "READ_ONLY"
)

// BizError is a business error with a stable code, user-readable message, and
// an action that tells the frontend how to recover (PRD §5.1).
type BizError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Action  Action `json:"action"`
}

func (e *BizError) Error() string {
	return fmt.Sprintf("[%s] %s (action=%s)", e.Code, e.Message, e.Action)
}

func New(code, message string, action Action) *BizError {
	return &BizError{Code: code, Message: message, Action: action}
}

// ---------------------------------------------------------------------------
// Pre-defined business errors (PRD §5.2 scenario matrix)
// ---------------------------------------------------------------------------

var (
	ErrAuthExpired    = New("AUTH_EXPIRED", "登录已过期，请重新登录", ActionReLogin)
	ErrAuthFailed     = New("AUTH_FAILED", "登录失败，请重试", ActionRetry)
	ErrForbidden      = New("FORBIDDEN", "没有权限执行此操作", ActionBackToRoom)
	ErrNotFound       = New("NOT_FOUND", "资源不存在", ActionBackToRoom)
	ErrGameFull       = New("GAME_FULL", "牌桌已满", ActionBackToRoom)
	ErrMembersLocked  = New("MEMBERS_LOCKED", "已开始记分，不能加入新成员", ActionBackToRoom)
	ErrInviteInvalid  = New("INVITE_INVALID", "邀请已失效", ActionBackToRoom)
	ErrGameNotForming = New("GAME_NOT_FORMING", "牌桌不在组桌阶段", ActionRefreshGame)
	ErrGameNotActive  = New("GAME_NOT_ACTIVE", "牌桌不在记分阶段", ActionRefreshGame)
	ErrGameEnded      = New("GAME_ENDED", "牌局已结束", ActionReadOnly)
	ErrRoundNotOpen   = New("ROUND_NOT_OPEN", "当前局不在提交阶段", ActionRefreshGame)
	ErrRoundNotReview = New("ROUND_NOT_REVIEW", "当前局不在复核阶段", ActionRefreshGame)
	ErrRoundNotReady  = New("ROUND_NOT_READY", "还有玩家未提交", ActionRefreshGame)
	ErrZeroSumFailed  = New("ZERO_SUM_FAILED", "本局总和不等于0，请检查并修改", ActionRetry)
	ErrRoundLocked    = New("ROUND_LOCKED", "本局已锁定", ActionRefreshGame)
	ErrGameNotEnded   = New("GAME_NOT_ENDED", "牌局未结束", ActionRefreshGame)
	ErrAdjustExpired  = New("ADJUSTMENT_EXPIRED", "积分调整已过期", ActionReadOnly)
	ErrAdjustResolved = New("ADJUSTMENT_RESOLVED", "积分调整已处理", ActionRefreshGame)
	ErrSeatOccupied   = New("SEAT_OCCUPIED", "该座位已有玩家，需对方同意才能互换", ActionRetry)
	ErrGameHasScores  = New("GAME_HAS_SCORES", "已有记分记录，请使用结束散台进行结算", ActionRetry)
	ErrAlreadyInGame  = New("ALREADY_IN_GAME", "你已有一张进行中的牌台，不能同时进多张台", ActionRetry)
	ErrSwapExpired    = New("SWAP_EXPIRED", "换位申请已过期", ActionRefreshGame)
	ErrSwapResolved   = New("SWAP_RESOLVED", "换位申请已处理", ActionRefreshGame)
	ErrSwapPending    = New("SWAP_PENDING", "你已有一个待处理的换位申请", ActionRetry)
	ErrSeatEmpty      = New("SEAT_EMPTY", "目标座位为空，可直接换座", ActionRetry)
	ErrInvalidInput   = New("INVALID_INPUT", "输入参数有误", ActionRetry)
	ErrInternal       = New("INTERNAL", "服务器内部错误", ActionRetry)
)
