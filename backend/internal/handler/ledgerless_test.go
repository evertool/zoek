package handler

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lk/zoek/backend/internal/store"
)

// ---------------------------------------------------------------------------
// 「无流水就删房间」统一口径
//
// 规则：牌局走到终态（取消 / 散台 / 失效）时，如果没有产生过任何流水
// （逐局记分提交 round_submissions 或 转分 score_adjustments，含待确认），
// 一律物理删除整个房间，不留「僵尸台」——否则记录页会冒出 0 笔账的空房间。
//
// 有流水则相反：房间必须保留，只能走散台结算。
// ---------------------------------------------------------------------------

// newGhostGame 造一个「终态且无流水」的僵尸台（当前状态直接置为目标状态）。
func newGhostGame(t *testing.T, r *gin.Engine, s *store.Store, code, status string) int64 {
	t.Helper()
	auth := loginAndAuth(t, r, code)
	w := doRequest(t, r, "POST", "/api/v1/games", auth, map[string]string{"request_id": "g-" + code})
	assertStatus(t, w, http.StatusCreated)
	gameID := int64(parseJSON(t, w)["game_id"].(float64))
	if status == "forming" {
		return gameID
	}
	if _, err := s.UpdateGameStatus(gameID, "forming", status); err != nil {
		t.Fatalf("造僵尸台 %s 失败: %v", status, err)
	}
	return gameID
}

func TestCancelGamePhysicallyRemovesRoom(t *testing.T) {
	r, _, s := testSetup(t)
	gameID, auth1, _ := createGameAndStart(t, r) // 已开局但一笔账都没有

	w := doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/cancel", gameID), auth1, map[string]string{"request_id": "c1"})
	assertStatus(t, w, http.StatusOK)

	if _, err := s.GetGame(gameID); err == nil {
		t.Fatal("取消开台后房间应被物理删除")
	}
}

// 未锁定局里的逐局提交同样算流水：以前只看「锁定局」，会把 already 记过的账
// 连着房间一起删掉，这里锁死。
func TestCancelBlockedByOpenRoundSubmission(t *testing.T) {
	r, _, s := testSetup(t)
	gameID, auth1, _ := createGameAndStart(t, r)

	w := doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d/rounds/current", gameID), auth1, nil)
	assertStatus(t, w, http.StatusOK)
	roundID := int64(parseJSON(t, w)["round_id"].(float64))

	// 只有一个人提交（局还没锁定、也没有转分）
	w = doRequest(t, r, "PUT", fmt.Sprintf("/api/v1/games/%d/rounds/%d/submission", gameID, roundID), auth1,
		map[string]interface{}{"score": 16, "request_id": "sub1"})
	assertStatus(t, w, http.StatusOK)

	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/cancel", gameID), auth1, map[string]string{"request_id": "c2"})
	assertStatus(t, w, http.StatusBadRequest)
	if code := parseJSON(t, w)["code"]; code != "GAME_HAS_SCORES" {
		t.Fatalf("code = %v, want GAME_HAS_SCORES", code)
	}
	if _, err := s.GetGame(gameID); err != nil {
		t.Fatal("有流水的房间不该被删掉")
	}
}

// 有流水的散台照旧：保留房间、状态置 ended。
func TestEndGameWithLedgerKeepsRoom(t *testing.T) {
	r, _, s := testSetup(t)
	gameID, auths := createGame4P(t, r)

	w := doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d/rounds/current", gameID), auths[0], nil)
	assertStatus(t, w, http.StatusOK)
	roundID := int64(parseJSON(t, w)["round_id"].(float64))
	playRound(t, r, gameID, roundID, auths, []int{16, -8, -4, -4})

	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/end", gameID), auths[0], map[string]string{"request_id": "e2"})
	assertStatus(t, w, http.StatusOK)

	game, err := s.GetGame(gameID)
	if err != nil {
		t.Fatalf("有流水的房间不该被删掉: %v", err)
	}
	if game.Status != "ended" {
		t.Fatalf("status = %q, want ended", game.Status)
	}
}

// 只有「待确认」转分（没锁定局、没生效转分）：算有流水 → 不删房间，
// 走既有的「无法结算」提示。
func TestPendingAdjustmentOnlyKeepsRoom(t *testing.T) {
	r, _, s := testSetup(t)
	gameID, auth1, _ := createGameAndStart(t, r)

	players, err := s.GetActiveGamePlayers(gameID)
	if err != nil || len(players) < 2 {
		t.Fatalf("取在座玩家失败: %v", err)
	}
	addAdjustment(t, s, gameID, players[0].ID, players[1].ID, "pending", "adj-pending-round")

	w := doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/end", gameID), auth1, map[string]string{"request_id": "e3"})
	assertStatus(t, w, http.StatusBadRequest)
	if code := parseJSON(t, w)["code"]; code != "NO_SCORE_RECORDS" {
		t.Fatalf("code = %v, want NO_SCORE_RECORDS", code)
	}
	if _, err := s.GetGame(gameID); err != nil {
		t.Fatal("有待确认流水（账单挂在台上）的房间不该被删掉")
	}
}

// 历史遗留的僵尸台（改动之前留下的 cancelled/expired/ended 且无流水）由清扫兜住；
// 有流水的一律不碰。
func TestPurgeLedgerlessTerminalGames(t *testing.T) {
	r, _, s := testSetup(t)

	cancelled := newGhostGame(t, r, s, "ghost-cancel", "cancelled")
	expired := newGhostGame(t, r, s, "ghost-expired", "expired")
	endedGhost := newGhostGame(t, r, s, "ghost-ended", "ended")

	// 有流水（转分）的散台：不许被清
	keeper, _, _ := createGameAndStart(t, r)
	kp, _ := s.GetActiveGamePlayers(keeper)
	addAdjustment(t, s, keeper, kp[0].ID, kp[1].ID, "accepted", "adj-keeper")
	if _, err := s.UpdateGameStatus(keeper, "active", "ended"); err != nil {
		t.Fatalf("置 ended 失败: %v", err)
	}

	purged, err := s.PurgeLedgerlessTerminalGames()
	if err != nil {
		t.Fatalf("清扫失败: %v", err)
	}
	if purged != 3 {
		t.Fatalf("清扫数量 = %d, want 3", purged)
	}
	for name, id := range map[string]int64{"cancelled": cancelled, "expired": expired, "ended": endedGhost} {
		if _, err := s.GetGame(id); err == nil {
			t.Errorf("%s 僵尸台应被物理删除，实际还在", name)
		}
	}
	if _, err := s.GetGame(keeper); err != nil {
		t.Errorf("有流水的牌局不该被清扫: %v", err)
	}
}
