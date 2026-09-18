package handler

import (
	"fmt"
	"net/http"
	"testing"
)

// TestGameDetailExposesHasLedger 房间页判断「能不能取消开台 / 要不要显示结束散台」
// 不再自己估算，直接用 GET /games/:id 的 has_ledger —— 与 CancelGame / EndGame 同一判据。
// （端上老算法漏了「有逐局提交但没锁定」，会出现「按钮能点、后端 400」的情况。）
func TestGameDetailExposesHasLedger(t *testing.T) {
	r, _, _ := testSetup(t)
	gameID, auths := createGame4P(t, r)

	// 新台：还没有任何流水账单 → 台主应看到「取消开台」
	w := doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d", gameID), auths[0], nil)
	assertStatus(t, w, http.StatusOK)
	if m := parseJSON(t, w); m["has_ledger"] != false {
		t.Fatalf("新台 has_ledger = %v, want false", m["has_ledger"])
	}

	// 给一笔分（转分即流水账单）→ true，台主改看「结束散台」
	w = doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d/rounds/current", gameID), auths[0], nil)
	assertStatus(t, w, http.StatusOK)
	roundID := int64(parseJSON(t, w)["round_id"].(float64))

	w = doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d", gameID), auths[0], nil)
	players := parseJSON(t, w)["players"].([]interface{})
	toID := int64(players[1].(map[string]interface{})["player_id"].(float64))

	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/rounds/%d/adjustments", gameID, roundID), auths[0],
		map[string]interface{}{
			"to_player_id":    toID,
			"adjustment_type": "supplement",
			"amount":          5,
			"auto_accept":     true,
			"request_id":      "hl-adj-1",
		})
	assertStatus(t, w, http.StatusCreated)

	w = doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d", gameID), auths[0], nil)
	assertStatus(t, w, http.StatusOK)
	if m := parseJSON(t, w); m["has_ledger"] != true {
		t.Fatalf("给分后 has_ledger = %v, want true", m["has_ledger"])
	}

	// 有流水后「取消开台」必须被后端拒绝（前端据此只显示「结束散台」）
	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/cancel", gameID), auths[0],
		map[string]string{"request_id": "hl-cancel"})
	assertStatus(t, w, http.StatusBadRequest)
}
