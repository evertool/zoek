package handler

import (
	"fmt"
	"net/http"
	"testing"
)

// 近7场胜场数必须与胜率同口径：胜 = 第 1 名且净分 > 0。
// bug 复盘：recent_wins 旧逻辑只判「净分 > 0」不看名次，
// 非头名的正分场会被误计成「胜」，出现「胜率 0% 但近7场 1胜」的自相矛盾。
func TestUserStatsRecentWinsConsistentWithWinRate(t *testing.T) {
	r, _, _ := testSetup(t)
	gameID, auths := createGame4P(t, r)

	// 唯一一局：p1 +8（第1名，胜）/ p2 +3（第2名，正分但非胜）/ p3 -5 / p4 -6
	w := doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d/rounds/current", gameID), auths[0], nil)
	assertStatus(t, w, http.StatusOK)
	roundID := int64(parseJSON(t, w)["round_id"].(float64))
	playRound(t, r, gameID, roundID, auths, []int{8, 3, -5, -6})

	assertStatus(t, doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/end", gameID), auths[0],
		map[string]string{"request_id": "end-stats"}), http.StatusOK)

	// p2：正分但非头名 → 胜率 0%，近7场也必须是 0 胜
	w = doRequest(t, r, "GET", "/api/v1/user/stats", auths[1], nil)
	assertStatus(t, w, http.StatusOK)
	st := parseJSON(t, w)
	if st["win_rate"].(float64) != 0 {
		t.Errorf("p2 win_rate = %v, want 0", st["win_rate"])
	}
	if st["recent_wins"].(float64) != 0 {
		t.Errorf("p2 recent_wins = %v, want 0（非头名的正分场不算胜）", st["recent_wins"])
	}

	// p1：头名且正分 → 胜率 100%，近7场 1 胜
	w = doRequest(t, r, "GET", "/api/v1/user/stats", auths[0], nil)
	assertStatus(t, w, http.StatusOK)
	st = parseJSON(t, w)
	if st["recent_wins"].(float64) != 1 {
		t.Errorf("p1 recent_wins = %v, want 1", st["recent_wins"])
	}
}
