package handler

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lk/zoek/backend/internal/model"
	"github.com/lk/zoek/backend/internal/store"
)

// 房间页长按座位：自己退出 / 台主移除雀友。
//
// 关键约束：只能软删除。round_submissions 与 score_adjustments 都有外键指向
// game_players(id)，物理删除会撞外键并丢掉历史记分归属，
// 所以离座 = status 置 'left' + seat 归零（释放座位）。

func gameDetail(t *testing.T, r *gin.Engine, auth string, gameID int64) map[string]interface{} {
	t.Helper()
	w := doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d", gameID), auth, nil)
	assertStatus(t, w, http.StatusOK)
	return parseJSON(t, w)
}

// playerBySeat 按座位号取玩家项（1-4）。
func playerBySeat(t *testing.T, detail map[string]interface{}, seat int) map[string]interface{} {
	t.Helper()
	players, _ := detail["players"].([]interface{})
	for _, raw := range players {
		if p, ok := raw.(map[string]interface{}); ok && int(p["seat"].(float64)) == seat {
			return p
		}
	}
	t.Fatalf("players 里找不到 seat=%d，实际：%v", seat, detail["players"])
	return nil
}

// playerByUserID 按 user_id 找玩家项（离座玩家也在 players 里）。
func playerByUserID(t *testing.T, detail map[string]interface{}, userID float64) map[string]interface{} {
	t.Helper()
	players, _ := detail["players"].([]interface{})
	for _, raw := range players {
		if p, ok := raw.(map[string]interface{}); ok && p["user_id"] == userID {
			return p
		}
	}
	t.Fatalf("players 里找不到 user_id=%v，实际：%v", userID, detail["players"])
	return nil
}

// ownerSeat 台主当前坐哪一号位（台主身份以后端的 creator_id 为准）。
func ownerSeat(t *testing.T, detail map[string]interface{}) int {
	t.Helper()
	creatorID := detail["creator_id"].(float64)
	return int(playerByUserID(t, detail, creatorID)["seat"].(float64))
}

// ---------------------------------------------------------------------------
// 自己退出
// ---------------------------------------------------------------------------

func TestLeaveGameFreesSeatAndMarksLeft(t *testing.T) {
	r, _, _ := testSetup(t)
	gameID, auths := createGame4P(t, r)

	before := gameDetail(t, r, auths[0], gameID)
	p2 := playerBySeat(t, before, 2)
	p2ID := p2["user_id"].(float64)

	w := doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/leave", gameID), auths[1], map[string]string{})
	assertStatus(t, w, http.StatusOK)
	if msg, _ := parseJSON(t, w)["message"].(string); msg == "" {
		t.Errorf("返回缺少 message")
	}

	detail := gameDetail(t, r, auths[0], gameID)
	if got := detail["player_count"].(float64); got != 3 {
		t.Errorf("player_count = %v, want 3（离座的人不该计入）", got)
	}

	p2After := playerByUserID(t, detail, p2ID)
	if p2After["status"] != "left" {
		t.Errorf("p2 status = %v, want left", p2After["status"])
	}
	if p2After["seat"].(float64) != 0 {
		t.Errorf("p2 seat = %v, want 0（座位要释放）", p2After["seat"])
	}
	if p2After["nickname"] == nil || p2After["nickname"] == "" {
		t.Errorf("离座玩家不该从 players 里消失——流水账单要靠他反查昵称")
	}

	// 释放出来的座位能被新人补上
	newAuth := loginAndAuth(t, r, "p5")
	w = doRequest(t, r, "POST", "/api/v1/games/join", newAuth,
		map[string]interface{}{"game_id": gameID, "request_id": "join-after-leave"})
	assertStatus(t, w, http.StatusCreated)

	after := gameDetail(t, r, auths[0], gameID)
	if got := after["player_count"].(float64); got != 4 {
		t.Errorf("补位后 player_count = %v, want 4", got)
	}
	// freeSeat 会优先复用刚释放的 2 号位，而且坐的必须是新人
	reused := playerBySeat(t, after, 2)
	if reused["status"] != "active" || reused["user_id"].(float64) == p2ID {
		t.Errorf("2 号位应被新人补上，实际 %v", reused)
	}
}

func TestLeaveGameTwiceRejected(t *testing.T) {
	r, _, _ := testSetup(t)
	gameID, auths := createGame4P(t, r)

	assertStatus(t, doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/leave", gameID), auths[1], map[string]string{}), http.StatusOK)

	// 再退一次：已经不是台上的人
	w := doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/leave", gameID), auths[1], map[string]string{})
	assertStatus(t, w, http.StatusBadRequest)
	if code := parseJSON(t, w)["code"]; code != "NOT_IN_GAME" {
		t.Errorf("code = %v, want NOT_IN_GAME", code)
	}

	// 离座后所有「需要成员身份」的接口都应被拒（GetGamePlayer 只看 active）
	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/swap_seat", gameID), auths[1], map[string]int{"target_seat": 1})
	if w.Code == http.StatusOK {
		t.Errorf("离座玩家不该还能换座（status=%d）", w.Code)
	}
}

// ---------------------------------------------------------------------------
// 台主退出 → 交棒；最后一人退出 → 散台
// ---------------------------------------------------------------------------

func TestLeaveGameTransfersOwnership(t *testing.T) {
	r, _, _ := testSetup(t)
	gameID, auths := createGame4P(t, r)

	oldOwnerID := gameDetail(t, r, auths[0], gameID)["creator_id"].(float64)
	w := doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/leave", gameID), auths[0], map[string]string{})
	assertStatus(t, w, http.StatusOK)

	detail := gameDetail(t, r, auths[1], gameID)
	creatorID := detail["creator_id"].(float64)
	if creatorID == oldOwnerID {
		t.Fatalf("台主退出后 creator_id 仍指向他——剩下的人会没人能结束散台")
	}
	if detail["player_count"].(float64) != 3 {
		t.Errorf("player_count = %v, want 3", detail["player_count"])
	}

	// 台主标要跟着走：有且只有一个 role=owner，且与 creator_id 一致
	owners := 0
	for _, raw := range detail["players"].([]interface{}) {
		p := raw.(map[string]interface{})
		if p["role"] != "owner" {
			continue
		}
		owners++
		if p["user_id"].(float64) != creatorID {
			t.Errorf("role=owner 的 user_id(%v) 与 creator_id(%v) 不一致", p["user_id"], creatorID)
		}
	}
	if owners != 1 {
		t.Errorf("owner 数量 = %d, want 1", owners)
	}

	// 接棒的人可以踢人
	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/kick", gameID), auths[1], map[string]int{"target_seat": 3})
	assertStatus(t, w, http.StatusOK)
}

func TestLeaveGameLastPlayerDissolves(t *testing.T) {
	r, _, _ := testSetup(t)
	gameID, auths := createGame4P(t, r)

	for _, auth := range auths {
		assertStatus(t, doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/leave", gameID), auth, map[string]string{}), http.StatusOK)
	}

	detail := gameDetail(t, r, auths[0], gameID)
	if st := detail["status"]; st != "cancelled" {
		t.Errorf("最后一人退出后 status = %v, want cancelled（不留僵尸台）", st)
	}
	if detail["player_count"].(float64) != 0 {
		t.Errorf("player_count = %v, want 0", detail["player_count"])
	}
}

// ---------------------------------------------------------------------------
// 台主踢人
// ---------------------------------------------------------------------------

func TestKickPlayer(t *testing.T) {
	r, _, _ := testSetup(t)
	gameID, auths := createGame4P(t, r)

	// 非台主不能踢
	w := doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/kick", gameID), auths[2], map[string]int{"target_seat": 2})
	assertStatus(t, w, http.StatusForbidden)
	if code := parseJSON(t, w)["code"]; code != "OWNER_ONLY" {
		t.Errorf("code = %v, want OWNER_ONLY", code)
	}

	p3ID := playerBySeat(t, gameDetail(t, r, auths[0], gameID), 3)["user_id"].(float64)
	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/kick", gameID), auths[0], map[string]int{"target_seat": 3})
	assertStatus(t, w, http.StatusOK)

	detail := gameDetail(t, r, auths[0], gameID)
	if got := detail["player_count"].(float64); got != 3 {
		t.Errorf("player_count = %v, want 3", got)
	}
	p3 := playerByUserID(t, detail, p3ID)
	if p3["status"] != "left" {
		t.Errorf("被踢玩家 status = %v, want left", p3["status"])
	}
	if p3["seat"].(float64) != 0 {
		t.Errorf("被踢玩家 seat = %v, want 0", p3["seat"])
	}

	// 已经空了的座位再踢 → 无人在座
	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/kick", gameID), auths[0], map[string]int{"target_seat": 3})
	assertStatus(t, w, http.StatusBadRequest)
	if code := parseJSON(t, w)["code"]; code != "TARGET_NOT_SEATED" {
		t.Errorf("code = %v, want TARGET_NOT_SEATED", code)
	}

	// 台主不能踢自己
	mySeat := ownerSeat(t, detail)
	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/kick", gameID), auths[0], map[string]int{"target_seat": mySeat})
	assertStatus(t, w, http.StatusBadRequest)
	if code := parseJSON(t, w)["code"]; code != "KICK_SELF" {
		t.Errorf("code = %v, want KICK_SELF", code)
	}

	// 被踢的人不能再操作牌台
	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/leave", gameID), auths[2], map[string]string{})
	assertStatus(t, w, http.StatusBadRequest)
}

// ---------------------------------------------------------------------------
// 有流水账单就不许离座（自己退出 / 台主移出都得走「结束散台」）
// ---------------------------------------------------------------------------

// addAdjustment 往牌局里塞一条转分（流水账单）。
func addAdjustment(t *testing.T, s *store.Store, gameID, from, to int64, status, reqID string) *model.ScoreAdjustment {
	t.Helper()
	adj := &model.ScoreAdjustment{
		GameID:         gameID,
		FromPlayerID:   from,
		ToPlayerID:     to,
		AdjustmentType: "supplement",
		Amount:         10,
		ProposedBy:     from,
		Status:         status,
		RequestID:      reqID,
		ExpiresAt:      time.Now().Add(time.Hour),
	}
	if err := s.CreateAdjustment(adj); err != nil {
		t.Fatalf("造转分失败: %v", err)
	}
	return adj
}

func TestLeaveAndKickBlockedWhenLedgerExists(t *testing.T) {
	r, _, s := testSetup(t)
	gameID, auths := createGame4P(t, r)

	players, err := s.GetActiveGamePlayers(gameID)
	if err != nil || len(players) < 2 {
		t.Fatalf("取在座玩家失败: %v", err)
	}
	addAdjustment(t, s, gameID, players[0].ID, players[1].ID, "accepted", "adj-ledger-1")

	// 自己退出被拒
	w := doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/leave", gameID), auths[1], map[string]string{})
	assertStatus(t, w, http.StatusBadRequest)
	if code := parseJSON(t, w)["code"]; code != "GAME_HAS_LEDGER" {
		t.Errorf("leave code = %v, want GAME_HAS_LEDGER", code)
	}

	// 台主踢人也被拒
	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/kick", gameID), auths[0], map[string]int{"target_seat": 2})
	assertStatus(t, w, http.StatusBadRequest)
	if code := parseJSON(t, w)["code"]; code != "GAME_HAS_LEDGER" {
		t.Errorf("kick code = %v, want GAME_HAS_LEDGER", code)
	}

	// 谁都没走掉，也没人变成 left。
	// 这里直接查 store，不走 GET /games/:id：那个接口内部会跑 5 小时超时扫描，
	// 而 SQLite 驱动把 MAX(created_at) 当字符串返回、扫不进 sql.NullTime（仅测试环境的问题），
	// 一旦牌局有转分就会 500 —— 生产 MySQL 没这个问题。
	stillSeated, err := s.GetActiveGamePlayers(gameID)
	if err != nil {
		t.Fatalf("取在座玩家失败: %v", err)
	}
	if len(stillSeated) != 4 {
		t.Errorf("被拒后在座人数 = %d, want 4（不该有人离座）", len(stillSeated))
	}
	all, _ := s.GetGamePlayers(gameID)
	for _, p := range all {
		if p.Status != model.PlayerStatusActive {
			t.Errorf("不该有人变成 left：player %d status=%s", p.ID, p.Status)
		}
		if p.Seat < 1 || p.Seat > 4 {
			t.Errorf("不该有人的座位被释放：player %d seat=%d", p.ID, p.Seat)
		}
	}
}

// 与上一条相对：流水账单**只拦离座**，换位任何时候都放行。
// 房间页长按他人座位 = 申请换位（台主也一样），不受流水账单影响；
// 只有「自己退出 / 台主移出」才是离座动作，必须走「结束散台」结算。
func TestSwapRequestAllowedWhenLedgerExists(t *testing.T) {
	r, _, s := testSetup(t)
	gameID, auths := createGame4P(t, r)

	players, err := s.GetActiveGamePlayers(gameID)
	if err != nil || len(players) < 2 {
		t.Fatalf("取在座玩家失败: %v", err)
	}
	addAdjustment(t, s, gameID, players[0].ID, players[1].ID, "accepted", "adj-ledger-swap")

	// 2 位向 1 位发起换位申请：有流水也照样放行
	swapBase := fmt.Sprintf("/api/v1/games/%d/swap_requests", gameID)
	w := doRequest(t, r, "POST", swapBase, auths[1], map[string]int{"target_seat": 1})
	assertStatus(t, w, http.StatusCreated)
	reqID := int64(parseJSON(t, w)["request"].(map[string]interface{})["id"].(float64))

	// 对方同意 → 座位号互换，各自的分数跟着人走
	w = doRequest(t, r, "POST", fmt.Sprintf("%s/%d/accept", swapBase, reqID), auths[0], map[string]string{})
	assertStatus(t, w, http.StatusOK)

	after, _ := s.GetActiveGamePlayers(gameID)
	for _, p := range after {
		if p.ID == players[1].ID && p.Seat != players[0].Seat {
			t.Errorf("2 位换位后 seat = %d, want %d", p.Seat, players[0].Seat)
		}
		if p.ID == players[0].ID && p.Seat != players[1].Seat {
			t.Errorf("1 位换位后 seat = %d, want %d", p.Seat, players[1].Seat)
		}
	}
	// 换位不是离座：谁都没走，也没人变成 left
	all, _ := s.GetGamePlayers(gameID)
	for _, p := range all {
		if p.Status != model.PlayerStatusActive {
			t.Errorf("换位不该让人离座：player %d status=%s", p.ID, p.Status)
		}
	}
}

// 「待确认」的转分同样算流水账单——账已经在台上了，人就别想走。
func TestLeaveBlockedByPendingAdjustment(t *testing.T) {
	r, _, s := testSetup(t)
	gameID, auths := createGame4P(t, r)

	players, _ := s.GetActiveGamePlayers(gameID)
	addAdjustment(t, s, gameID, players[0].ID, players[1].ID, "pending", "adj-pending-block")

	w := doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/leave", gameID), auths[1], map[string]string{})
	assertStatus(t, w, http.StatusBadRequest)
	if code := parseJSON(t, w)["code"]; code != "GAME_HAS_LEDGER" {
		t.Errorf("code = %v, want GAME_HAS_LEDGER", code)
	}
}

// 离座会顺手清掉该玩家参与的「待确认」转分。
// 正常流程走不到这里（上面两条用例证明有流水就不给离座），保留是为了兜住
// 「count 检查通过之后、离座落库之前」的并发窗口 —— 所以直接在 store 层验证组合行为。
func TestCancelPendingAdjustmentsOnLeave(t *testing.T) {
	r, _, s := testSetup(t)
	gameID, _ := createGame4P(t, r)

	players, _ := s.GetActiveGamePlayers(gameID)
	from, to := players[0], players[1]
	adj := addAdjustment(t, s, gameID, from.ID, to.ID, "pending", "adj-race-1")

	if err := s.CancelPendingAdjustmentsForPlayer(gameID, to.ID); err != nil {
		t.Fatalf("取消待确认失败: %v", err)
	}
	if err := s.LeaveGamePlayer(gameID, to.ID); err != nil {
		t.Fatalf("离座失败: %v", err)
	}

	got, err := s.GetAdjustment(adj.ID)
	if err != nil {
		t.Fatalf("回查转分失败: %v", err)
	}
	if got.Status != "cancelled" {
		t.Errorf("待确认转分 status = %q, want cancelled", got.Status)
	}
}

// ---------------------------------------------------------------------------
// 首页「进行中牌台」列表与离座的关系
//
// bug 复盘：GetActiveGames 的子查询原本没过滤 game_players.status，
// 被台主踢出 / 自己退出后留下的 status=left 行仍会命中，
// 导致首页还显示一张已经不在座的牌台。此处锁死回归。
// ---------------------------------------------------------------------------

func activeGameIDs(t *testing.T, r *gin.Engine, auth string) []float64 {
	t.Helper()
	w := doRequest(t, r, "GET", "/api/v1/games/active", auth, nil)
	assertStatus(t, w, http.StatusOK)
	games, _ := parseJSON(t, w)["games"].([]interface{})
	ids := make([]float64, 0, len(games))
	for _, raw := range games {
		if g, ok := raw.(map[string]interface{}); ok {
			ids = append(ids, g["game_id"].(float64))
		}
	}
	return ids
}

func containsID(ids []float64, id float64) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

func TestActiveGamesHidesSelfLeftGame(t *testing.T) {
	r, _, _ := testSetup(t)
	gameID, auths := createGame4P(t, r)

	if !containsID(activeGameIDs(t, r, auths[1]), float64(gameID)) {
		t.Fatalf("前置失败：入局者的进行中列表应包含 game %d", gameID)
	}

	assertStatus(t, doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/leave", gameID), auths[1], map[string]string{}), http.StatusOK)

	if ids := activeGameIDs(t, r, auths[1]); containsID(ids, float64(gameID)) {
		t.Errorf("自己退出后首页仍显示进行中牌台：%v", ids)
	}
	// 台主仍在座，列表不受影响
	if ids := activeGameIDs(t, r, auths[0]); !containsID(ids, float64(gameID)) {
		t.Errorf("台主的进行中列表丢了 game %d：%v", gameID, ids)
	}
}

func TestActiveGamesHidesKickedGame(t *testing.T) {
	r, _, _ := testSetup(t)
	gameID, auths := createGame4P(t, r)

	p3ID := playerBySeat(t, gameDetail(t, r, auths[0], gameID), 3)["user_id"].(float64)
	assertStatus(t, doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/kick", gameID), auths[0],
		map[string]interface{}{"target_player_id": p3ID}), http.StatusOK)

	if ids := activeGameIDs(t, r, auths[2]); containsID(ids, float64(gameID)) {
		t.Errorf("被台主踢出后首页仍显示进行中牌台：%v", ids)
	}
	if ids := activeGameIDs(t, r, auths[0]); !containsID(ids, float64(gameID)) {
		t.Errorf("台主的进行中列表丢了 game %d：%v", gameID, ids)
	}
}
