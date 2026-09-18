package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lk/zoek/backend/internal/errs"
	"github.com/lk/zoek/backend/internal/middleware"
	"github.com/lk/zoek/backend/internal/model"
	"github.com/lk/zoek/backend/internal/store"
	"github.com/lk/zoek/backend/internal/ws"
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
	// Tags：可多选的给分标签 code（自摸/明杠/暗杠/杠爆/抢杠），见 allowedAdjustmentTags
	Tags []string `json:"tags"`
	// AutoAccept：给分早就一律直接生效，这个字段只为兼容老版本小程序保留，服务端不再读取。
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
	Tags           []string  `json:"tags"`
	Status         string    `json:"status"`
	ExpiresAt      time.Time `json:"expires_at"`
	CreatedAt      time.Time `json:"created_at"`
}

// allowedAdjustmentTags 给分标签白名单。code 与小程序 utils/score-tags.js 的 SCORE_TAGS
// 一一对应：自摸 zimo / 明杠 minggang / 暗杠 angang / 放杠 fanggang / 杠爆 gangbao / 抢杠 qianggang。
// 落到 score_adjustments.tags 的是 code 的逗号串，展示文案由端上映射（后端不碰中文）。
var allowedAdjustmentTags = map[string]bool{
	"zimo":      true,
	"minggang":  true,
	"angang":    true,
	"fanggang":  true,
	"gangbao":   true,
	"qianggang": true,
}

// normalizeAdjustmentTags 去空、去重、白名单校验；遇到不认识的 code 直接报错，
// 避免脏标签进库（端上加了新标签忘了同步这里时，接口会明确失败而不是静默吞掉）。
func normalizeAdjustmentTags(tags []string) ([]string, error) {
	out := make([]string, 0, len(tags))
	seen := map[string]bool{}
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" || seen[t] {
			continue
		}
		if !allowedAdjustmentTags[t] {
			return nil, fmt.Errorf("unknown adjustment tag: %s", t)
		}
		seen[t] = true
		out = append(out, t)
	}
	return out, nil
}

// joinAdjustmentTags / splitAdjustmentTags：库里存逗号串，接口进出都是数组
func joinAdjustmentTags(tags []string) string {
	return strings.Join(tags, ",")
}

func splitAdjustmentTags(s string) []string {
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
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

	// 散台一律拦截给分：牌局结束后积分已结算，不再接受任何转分/补退分。
	// （原 PRD §3.2 rule 10 的「散台后 24h 补退分窗口」已下线，用户确认不留）
	if game.Status != "active" {
		c.JSON(http.StatusBadRequest, errs.ErrGameEnded)
		return
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

	// 局概念已移除：转分不再关联局（RoundID 落 0，兼容旧数据列）
	req.RoundID = 0

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

	// 给分标签（可多选）：只认白名单里的 code
	tags, tagErr := normalizeAdjustmentTags(req.Tags)
	if tagErr != nil {
		c.JSON(http.StatusBadRequest, errs.New("INVALID_TAG", "给分标签不被支持", errs.ActionRetry))
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

	requestID := req.RequestID
	if requestID == "" {
		requestID = fmt.Sprintf("adj-%d-%d-%d", gameID, req.RoundID, time.Now().UnixNano())
	}

	now := time.Now()
	resolvedAt := now
	// 给分（转分）一律直接生效：没有「待确认 → 对方接受/驳回」这套审核流了，
	// 也没有补分申请，落库就是 accepted。（DTO 里的 auto_accept 只为兼容老版本小程序，服务端忽略。）
	adj := &model.ScoreAdjustment{
		GameID:         gameID,
		RoundID:        req.RoundID,
		FromPlayerID:   fromPlayer.ID,
		ToPlayerID:     req.ToPlayerID,
		AdjustmentType: req.AdjustmentType,
		Amount:         req.Amount,
		Reason:         req.Reason,
		Tags:           joinAdjustmentTags(tags),
		ProposedBy:     fromPlayer.ID,
		Status:         "accepted",
		ResolvedAt:     &resolvedAt,
		ResolvedBy:     &userID,
		RequestID:      requestID,
		ExpiresAt:      now.Add(24 * time.Hour),
	}

	if err := h.Store.CreateAdjustment(adj); err != nil {
		c.JSON(http.StatusInternalServerError, errs.ErrInternal)
		return
	}

	// 给分动画广播：推给全台玩家（含发起人），前端各自播放筹码飞行动画。
	// 注意 middleware.RoomSyncBroadcast 的 "game" 信号只带全量刷新不带明细，这里需要携带
	// from/to/amount 才能让每台手机知道筹码从谁飞向谁。
	ws.Emit(gameID, "give", gin.H{
		"id":             adj.ID,
		"from_player_id": adj.FromPlayerID,
		"to_player_id":   adj.ToPlayerID,
		"amount":         req.Amount,
	})

	// Find target player nickname for message
	var toNickname string
	for _, p := range players {
		if p.ID == req.ToPlayerID {
			toNickname = p.NicknameSnapshot
			break
		}
	}

	message := fmt.Sprintf("已转记 %d 分给 %s", req.Amount, toNickname)

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
			Tags:           splitAdjustmentTags(adj.Tags),
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
			Tags:           splitAdjustmentTags(a.Tags),
			Status:         a.Status,
			ExpiresAt:      a.ExpiresAt,
			CreatedAt:      a.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"adjustments": result})
}
