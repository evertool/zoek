package handler

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/lk/zoek/backend/internal/model"
)

// 给分标签（自摸/明杠/暗杠/杠爆/抢杠）可多选：
// 请求/响应都是数组，落库为逗号串，流水接口能原样读回；重复标签会去重。
func TestAdjustmentTagsRoundTrip(t *testing.T) {
	r, _, _ := testSetup(t)
	gameID, auth1, auth2 := createGameAndStart(t, r)

	// 当前局 + 收分方 player_id
	w := doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d/rounds/current", gameID), auth1, nil)
	assertStatus(t, w, http.StatusOK)
	roundID := int64(parseJSON(t, w)["round_id"].(float64))

	w = doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d", gameID), auth1, nil)
	assertStatus(t, w, http.StatusOK)
	var toPlayerID int64
	for _, p := range parseJSON(t, w)["players"].([]interface{}) {
		pMap := p.(map[string]interface{})
		if pMap["role"] == "player" {
			toPlayerID = int64(pMap["player_id"].(float64))
		}
	}
	if toPlayerID == 0 {
		t.Fatal("no peer player found")
	}

	// 台间快捷给分（auto_accept）带多个标签，其中一个重复 → 去重后按传入顺序 3 个
	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/rounds/%d/adjustments", gameID, roundID), auth1,
		map[string]interface{}{
			"to_player_id":    toPlayerID,
			"adjustment_type": "supplement",
			"amount":          10,
			"auto_accept":     true,
			"tags":            []string{"zimo", "fanggang", "qianggang", "zimo"},
			"request_id":      "tag-adj-1",
		})
	assertStatus(t, w, http.StatusCreated)
	created := parseJSON(t, w)["adjustment"].(map[string]interface{})
	tags := toStrings(t, created["tags"])
	if len(tags) != 3 || tags[0] != "zimo" || tags[1] != "fanggang" || tags[2] != "qianggang" {
		t.Fatalf("create response tags = %v, want [zimo fanggang qianggang]", tags)
	}

	// 流水（对局内所有人可读）：读回同一组标签
	w = doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d/adjustments", gameID), auth2, nil)
	assertStatus(t, w, http.StatusOK)
	list := parseJSON(t, w)["adjustments"].([]interface{})
	if len(list) != 1 {
		t.Fatalf("adjustments len = %d, want 1", len(list))
	}
	got := toStrings(t, list[0].(map[string]interface{})["tags"])
	if len(got) != 3 || got[0] != "zimo" || got[1] != "fanggang" || got[2] != "qianggang" {
		t.Fatalf("ledger tags = %v, want [zimo fanggang qianggang]", got)
	}

	// 不带标签的老用法：tags 为空数组（不是 null，端上不用额外判空）
	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/rounds/%d/adjustments", gameID, roundID), auth1,
		map[string]interface{}{
			"to_player_id":    toPlayerID,
			"adjustment_type": "supplement",
			"amount":          3,
			"auto_accept":     true,
			"request_id":      "tag-adj-2",
		})
	assertStatus(t, w, http.StatusCreated)
	if tags = toStrings(t, parseJSON(t, w)["adjustment"].(map[string]interface{})["tags"]); len(tags) != 0 {
		t.Fatalf("no-tag adjustment tags = %v, want empty", tags)
	}
}

// 不在白名单里的标签要明确拒绝，别静默落库。
func TestAdjustmentTagsRejectsUnknown(t *testing.T) {
	r, _, _ := testSetup(t)
	gameID, auth1, _ := createGameAndStart(t, r)

	w := doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d/rounds/current", gameID), auth1, nil)
	roundID := int64(parseJSON(t, w)["round_id"].(float64))
	w = doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d", gameID), auth1, nil)
	var toPlayerID int64
	for _, p := range parseJSON(t, w)["players"].([]interface{}) {
		pMap := p.(map[string]interface{})
		if pMap["role"] == "player" {
			toPlayerID = int64(pMap["player_id"].(float64))
		}
	}

	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/rounds/%d/adjustments", gameID, roundID), auth1,
		map[string]interface{}{
			"to_player_id":    toPlayerID,
			"adjustment_type": "supplement",
			"amount":          10,
			"auto_accept":     true,
			"tags":            []string{"zimo", "not-a-real-tag"},
			"request_id":      "tag-adj-bad",
		})
	assertStatus(t, w, http.StatusBadRequest)
	if m := parseJSON(t, w); m["code"] != "INVALID_TAG" {
		t.Fatalf("unknown tag code = %v, want INVALID_TAG", m["code"])
	}
}

// toStrings 把接口返回的 []interface{} 拉平成 []string。
func toStrings(t *testing.T, v interface{}) []string {
	t.Helper()
	arr, ok := v.([]interface{})
	if !ok {
		t.Fatalf("tags is %T, want array", v)
	}
	out := make([]string, 0, len(arr))
	for _, it := range arr {
		out = append(out, fmt.Sprint(it))
	}
	return out
}

// 历史战绩详情（/games/:id/history）的给分流水也必须带 tags，
// 否则战绩页「自摸/明杠」等标签不显示（回归缺陷：GetHistoryDetail 漏带 Tags 字段）。
func TestHistoryDetailIncludesTags(t *testing.T) {
	r, _, s := testSetup(t)
	gameID, auth1, auth2 := createGameAndStart(t, r)

	w := doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d/rounds/current", gameID), auth1, nil)
	assertStatus(t, w, http.StatusOK)
	roundID := int64(parseJSON(t, w)["round_id"].(float64))
	w = doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d", gameID), auth1, nil)
	var toPlayerID int64
	for _, p := range parseJSON(t, w)["players"].([]interface{}) {
		pMap := p.(map[string]interface{})
		if pMap["role"] == "player" {
			toPlayerID = int64(pMap["player_id"].(float64))
		}
	}

	// 带标签的快捷给分（即时生效）
	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/rounds/%d/adjustments", gameID, roundID), auth1,
		map[string]interface{}{
			"to_player_id":    toPlayerID,
			"adjustment_type": "supplement",
			"amount":          8,
			"auto_accept":     true,
			"tags":            []string{"zimo", "angang"},
			"request_id":      "hist-tag-1",
		})
	assertStatus(t, w, http.StatusCreated)

	// 强制散台后看历史详情
	if err := s.DB.Model(&model.Game{}).Where("id = ?", gameID).Update("status", "ended").Error; err != nil {
		t.Fatalf("force end game: %v", err)
	}
	w = doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d/history", gameID), auth2, nil)
	assertStatus(t, w, http.StatusOK)
	adjs := parseJSON(t, w)["adjustments"].([]interface{})
	if len(adjs) != 1 {
		t.Fatalf("history adjustments len = %d, want 1", len(adjs))
	}
	tags := toStrings(t, adjs[0].(map[string]interface{})["tags"])
	if len(tags) != 2 || tags[0] != "zimo" || tags[1] != "angang" {
		t.Fatalf("history detail tags = %v, want [zimo angang]", tags)
	}
}
