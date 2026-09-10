package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lk/zoek/backend/internal/logger"
	"github.com/lk/zoek/backend/internal/middleware"
	"github.com/lk/zoek/backend/internal/store"
	"github.com/lk/zoek/backend/pkg/wechat"
	"gorm.io/driver/sqlite" // test-only: in-memory SQLite for fast tests
	"gorm.io/gorm"
)

// testSetup creates a fully wired test environment with an in-memory SQLite DB.
// Note: production uses MySQL exclusively; SQLite is test-only for speed.
func testSetup(t *testing.T) (*gin.Engine, *middleware.JWTManager, *store.Store) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	s := store.New(db, logger.NewNop())
	if err := s.AutoMigrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	jwt := middleware.NewJWTManager("test-secret", 0)
	r := gin.New()
	r.Use(middleware.RequestID())
	r.Use(middleware.ErrorHandler(logger.NewNop()))

	authH := NewAuthHandler(s, jwt, wechat.NewMockClient(func(code string) (string, string, error) {
		return "wx_openid_" + code, "", nil
	}))
	gameH := NewGameHandler(s, jwt, wechat.NewMockQRClient(
		func(code string) (string, string, error) { return "wx_openid_" + code, "", nil },
		func(page, scene string) ([]byte, error) { return []byte("mock-png-data"), nil },
	))
	roundH := NewRoundHandler(s)
	adjH := NewAdjustmentHandler(s)
	swapH := NewSwapHandler(s)
	settleH := NewSettlementHandler(s)
	lbH := NewLeaderboardHandler(s)

	v1 := r.Group("/api/v1")
	{
		v1.POST("/auth/login", authH.Login)
		auth := v1.Group("")
		auth.Use(jwt.Auth())
		{
			auth.GET("/user/profile", authH.GetProfile)
			auth.PUT("/user/profile", authH.UpdateProfile)

			auth.POST("/games", gameH.CreateGame)
			auth.GET("/games/active", gameH.GetActiveGames)
			auth.GET("/games/history", gameH.GetHistoryGames)
			auth.GET("/games/:game_id", gameH.GetGame)
			auth.POST("/games/join", gameH.JoinGame)
			auth.POST("/games/:game_id/start", gameH.StartGame)
			auth.POST("/games/:game_id/cancel", gameH.CancelGame)
			auth.POST("/games/:game_id/end", gameH.EndGame)
			auth.POST("/games/:game_id/hide", gameH.HideGame)
			auth.POST("/games/:game_id/swap_seat", gameH.SwapSeat)
			auth.POST("/games/:game_id/swap_requests", swapH.CreateSwapRequest)
			auth.GET("/games/:game_id/swap_requests/pending", swapH.GetPendingSwapRequest)
			auth.POST("/games/:game_id/swap_requests/:id/:action", swapH.ResolveSwapRequest)
			auth.GET("/rank/me", NewRankHandler(s).GetMyRank)

			auth.POST("/games/:game_id/rounds", roundH.CreateNextRound)
			auth.GET("/games/:game_id/rounds/current", roundH.GetCurrentRound)
			auth.PUT("/games/:game_id/rounds/:round_id/submission", roundH.SubmitScore)
			auth.POST("/games/:game_id/rounds/:round_id/lock", roundH.LockRound)
			auth.POST("/games/:game_id/rounds/:round_id/next", roundH.CreateNextRound)
			auth.POST("/games/:game_id/rounds/manual-next", roundH.ManualNextRound)
			auth.GET("/games/:game_id/rounds/:round_id", roundH.GetRoundDetail)

			auth.POST("/games/:game_id/rounds/:round_id/adjustments", adjH.CreateAdjustment)
			auth.GET("/games/:game_id/adjustments", adjH.ListAdjustments)
			auth.POST("/games/:game_id/adjustments/:adjustment_id/accept", adjH.AcceptAdjustment)
			auth.POST("/games/:game_id/adjustments/:adjustment_id/reject", adjH.RejectAdjustment)
			auth.POST("/games/:game_id/adjustments/:adjustment_id/cancel", adjH.CancelAdjustment)

			auth.GET("/games/:game_id/settlement", settleH.GetSettlement)
			auth.GET("/games/:game_id/history", settleH.GetHistoryDetail)

			auth.GET("/leaderboard", lbH.GetLeaderboard)
			auth.GET("/user/stats", lbH.GetUserStats)
		}
	}
	return r, jwt, s
}

// loginAndAuth creates a user via login, returns the auth header.
func loginAndAuth(t *testing.T, r *gin.Engine, code string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"code": code, "nickname": "玩家" + code})
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	token := resp["token"].(string)
	return "Bearer " + token
}

func doRequest(t *testing.T, r *gin.Engine, method, path, auth string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func assertStatus(t *testing.T, w *httptest.ResponseRecorder, expected int) {
	t.Helper()
	if w.Code != expected {
		t.Fatalf("status = %d, want %d, body: %s", w.Code, expected, w.Body.String())
	}
}

func parseJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("parse json: %v, body: %s", err, w.Body.String())
	}
	return m
}

// ---------------------------------------------------------------------------
// Auth tests
// ---------------------------------------------------------------------------

func TestLogin(t *testing.T) {
	r, _, _ := testSetup(t)

	auth := loginAndAuth(t, r, "u1")
	if auth == "" {
		t.Fatal("empty token")
	}

	// Get profile
	w := doRequest(t, r, "GET", "/api/v1/user/profile", auth, nil)
	assertStatus(t, w, http.StatusOK)
	m := parseJSON(t, w)
	if m["nickname"] != "玩家u1" {
		t.Fatalf("nickname = %v, want 玩家u1", m["nickname"])
	}
}

func TestLoginDuplicate(t *testing.T) {
	r, _, _ := testSetup(t)
	auth1 := loginAndAuth(t, r, "dup")
	auth2 := loginAndAuth(t, r, "dup")
	if auth1 == "" || auth2 == "" {
		t.Fatal("empty token")
	}

	// Both should get the same user ID
	w1 := doRequest(t, r, "GET", "/api/v1/user/profile", auth1, nil)
	m1 := parseJSON(t, w1)
	w2 := doRequest(t, r, "GET", "/api/v1/user/profile", auth2, nil)
	m2 := parseJSON(t, w2)
	if m1["user_id"] != m2["user_id"] {
		t.Fatalf("same openid should return same user_id: %v vs %v", m1["user_id"], m2["user_id"])
	}
}

// TestProfilePersistence verifies that a user who saved nickname and avatar
// does NOT get need_profile=true on the next login (PRD §4.2-A 完善资料只弹一次).
func TestProfilePersistence(t *testing.T) {
	r, _, _ := testSetup(t)

	loginBody := map[string]string{"code": "prof"}

	// First login: fresh user needs profile
	w := doRequest(t, r, "POST", "/api/v1/auth/login", "", loginBody)
	assertStatus(t, w, http.StatusOK)
	m := parseJSON(t, w)
	if m["need_profile"] != true {
		t.Fatalf("fresh user need_profile = %v, want true", m["need_profile"])
	}
	if m["avatar_url"] != "" {
		t.Fatalf("fresh user avatar_url = %v, want empty", m["avatar_url"])
	}
	token := "Bearer " + m["token"].(string)

	// Save profile (nickname + relative-path avatar)
	w = doRequest(t, r, "PUT", "/api/v1/user/profile", token,
		map[string]string{"nickname": "阿强", "avatar_url": "/uploads/avatars/test_avatar.jpg"})
	assertStatus(t, w, http.StatusOK)
	m = parseJSON(t, w)
	if m["need_profile"] != false {
		t.Fatalf("after save need_profile = %v, want false", m["need_profile"])
	}

	// Re-login: profile must be complete, no need to fill again
	w = doRequest(t, r, "POST", "/api/v1/auth/login", "", loginBody)
	assertStatus(t, w, http.StatusOK)
	m = parseJSON(t, w)
	if m["need_profile"] != false {
		t.Fatalf("re-login need_profile = %v, want false", m["need_profile"])
	}
	if m["nickname"] != "阿强" {
		t.Fatalf("re-login nickname = %v, want 阿强", m["nickname"])
	}
	if m["avatar_url"] != "/uploads/avatars/test_avatar.jpg" {
		t.Fatalf("re-login avatar_url = %v, want /uploads/avatars/test_avatar.jpg", m["avatar_url"])
	}
}

// TestProfileEmptyAvatarNotComplete verifies that saving only a nickname
// (without avatar) still leaves need_profile=true.
func TestProfileEmptyAvatarNotComplete(t *testing.T) {
	r, _, _ := testSetup(t)

	w := doRequest(t, r, "POST", "/api/v1/auth/login", "", map[string]string{"code": "noav"})
	assertStatus(t, w, http.StatusOK)
	m := parseJSON(t, w)
	token := "Bearer " + m["token"].(string)

	// Save nickname only (no avatar)
	w = doRequest(t, r, "PUT", "/api/v1/user/profile", token,
		map[string]string{"nickname": "阿明", "avatar_url": ""})
	assertStatus(t, w, http.StatusOK)
	m = parseJSON(t, w)
	if m["need_profile"] != true {
		t.Fatalf("after saving nickname only need_profile = %v, want true", m["need_profile"])
	}

	// Re-login: profile still incomplete
	w = doRequest(t, r, "POST", "/api/v1/auth/login", "", map[string]string{"code": "noav"})
	assertStatus(t, w, http.StatusOK)
	m = parseJSON(t, w)
	if m["need_profile"] != true {
		t.Fatalf("re-login without avatar need_profile = %v, want true", m["need_profile"])
	}
}

// ---------------------------------------------------------------------------
// Game lifecycle tests
// ---------------------------------------------------------------------------

// createGameAndStart creates a game, has a second user join, and starts it.
func createGameAndStart(t *testing.T, r *gin.Engine) (int64, string, string) {
	t.Helper()
	auth1 := loginAndAuth(t, r, "creator")
	auth2 := loginAndAuth(t, r, "joiner")

	// Create game
	w := doRequest(t, r, "POST", "/api/v1/games", auth1, map[string]string{"name": "今晚麻将", "request_id": "r1"})
	assertStatus(t, w, http.StatusCreated)
	m := parseJSON(t, w)
	gameID := int64(m["game_id"].(float64))
	token := m["invite_token"].(string)

	// Join
	w = doRequest(t, r, "POST", "/api/v1/games/join", auth2, map[string]string{"invite_token": token, "request_id": "r2"})
	assertStatus(t, w, http.StatusCreated)

	// Start
	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/start", gameID), auth1, map[string]string{"request_id": "r3"})
	assertStatus(t, w, http.StatusOK)

	return gameID, auth1, auth2
}

func TestCreateGame(t *testing.T) {
	r, _, _ := testSetup(t)
	auth := loginAndAuth(t, r, "creator")

	w := doRequest(t, r, "POST", "/api/v1/games", auth, map[string]string{"name": "测试牌局", "request_id": "r1"})
	assertStatus(t, w, http.StatusCreated)
	m := parseJSON(t, w)
	if m["name"] != "测试牌局" {
		t.Fatalf("name = %v", m["name"])
	}
	if m["status"] != "forming" {
		t.Fatalf("status = %v", m["status"])
	}
	if m["invite_token"] == "" || m["invite_token"] == nil {
		t.Fatal("missing invite_token")
	}
}

func TestCreateGameDefaultName(t *testing.T) {
	r, _, _ := testSetup(t)
	auth := loginAndAuth(t, r, "creator")

	w := doRequest(t, r, "POST", "/api/v1/games", auth, map[string]string{"request_id": "r1"})
	assertStatus(t, w, http.StatusCreated)
	m := parseJSON(t, w)
	// PRD v1.0 §4.2-A: 空台名自动生成"得闲开台 M月D日"
	if m["name"].(string) == "" {
		t.Fatal("name should not be empty")
	}
}

func TestJoinGameAlreadyJoined(t *testing.T) {
	r, _, _ := testSetup(t)
	auth := loginAndAuth(t, r, "creator")

	w := doRequest(t, r, "POST", "/api/v1/games", auth, map[string]string{"request_id": "r1"})
	m := parseJSON(t, w)
	token := m["invite_token"].(string)

	// Creator tries to join again
	w = doRequest(t, r, "POST", "/api/v1/games/join", auth, map[string]string{"invite_token": token, "request_id": "r2"})
	assertStatus(t, w, http.StatusOK)
}

func TestSwapSeat(t *testing.T) {
	r, _, _ := testSetup(t)
	creator := loginAndAuth(t, r, "creator")
	joiner := loginAndAuth(t, r, "joiner")

	w := doRequest(t, r, "POST", "/api/v1/games", creator, map[string]string{"request_id": "r1"})
	assertStatus(t, w, http.StatusCreated)
	inviteToken := parseJSON(t, w)["invite_token"].(string)

	// 第二位玩家加入（凑满 2 人自动开局）
	w = doRequest(t, r, "POST", "/api/v1/games/join", joiner, map[string]string{"invite_token": inviteToken, "request_id": "r2"})
	assertStatus(t, w, http.StatusCreated)
	gameID := int64(parseJSON(t, w)["game_id"].(float64))

	// joiner 在 2 号位，长按换到空位 4：立即生效，无需申请
	path := fmt.Sprintf("/api/v1/games/%d/swap_seat", gameID)
	w = doRequest(t, r, "POST", path, joiner, map[string]int{"target_seat": 4})
	assertStatus(t, w, http.StatusOK)
	if m := parseJSON(t, w); m["seat"].(float64) != 4 {
		t.Fatalf("seat = %v, want 4", m["seat"])
	}

	// 换到已占用的 1 号位：需要对方同意，接口直接拒绝
	w = doRequest(t, r, "POST", path, joiner, map[string]int{"target_seat": 1})
	assertStatus(t, w, http.StatusBadRequest)
	if m := parseJSON(t, w); m["code"] != "SEAT_OCCUPIED" {
		t.Fatalf("code = %v, want SEAT_OCCUPIED", m["code"])
	}

	// 非法座位号
	w = doRequest(t, r, "POST", path, joiner, map[string]int{"target_seat": 5})
	assertStatus(t, w, http.StatusBadRequest)

	// 换位后玩家列表按座位号排列：1 号位 creator 在前，4 号位 joiner 在后
	w = doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d", gameID), joiner, nil)
	assertStatus(t, w, http.StatusOK)
	players := parseJSON(t, w)["players"].([]interface{})
	if len(players) != 2 {
		t.Fatalf("players = %d, want 2", len(players))
	}
	if first := players[0].(map[string]interface{}); first["nickname"] != "玩家creator" {
		t.Fatalf("first player = %v, want 玩家creator", first["nickname"])
	}
}

// TestSeatSwapRequest 覆盖「长按他人座位 → 申请 → 对方待处理 → 同意 → 座位互换」。
func TestSeatSwapRequest(t *testing.T) {
	r, _, _ := testSetup(t)
	creator := loginAndAuth(t, r, "creator")
	joiner := loginAndAuth(t, r, "joiner")

	w := doRequest(t, r, "POST", "/api/v1/games", creator, map[string]string{"request_id": "r1"})
	assertStatus(t, w, http.StatusCreated)
	inviteToken := parseJSON(t, w)["invite_token"].(string)
	w = doRequest(t, r, "POST", "/api/v1/games/join", joiner, map[string]string{"invite_token": inviteToken, "request_id": "r2"})
	assertStatus(t, w, http.StatusCreated)
	gameID := int64(parseJSON(t, w)["game_id"].(float64))

	// joiner（2位）申请与 creator（1位）互换
	base := fmt.Sprintf("/api/v1/games/%d/swap_requests", gameID)
	w = doRequest(t, r, "POST", base, joiner, map[string]int{"target_seat": 1})
	assertStatus(t, w, http.StatusCreated)
	reqID := int64(parseJSON(t, w)["request"].(map[string]interface{})["id"].(float64))

	// 重复申请应被拒
	w = doRequest(t, r, "POST", base, joiner, map[string]int{"target_seat": 1})
	assertStatus(t, w, http.StatusBadRequest)
	if m := parseJSON(t, w); m["code"] != "SWAP_PENDING" {
		t.Fatalf("code = %v, want SWAP_PENDING", m["code"])
	}

	// 目标座位为空：提示直接换座
	w = doRequest(t, r, "POST", base, joiner, map[string]int{"target_seat": 3})
	assertStatus(t, w, http.StatusBadRequest)
	if m := parseJSON(t, w); m["code"] != "SEAT_EMPTY" {
		t.Fatalf("code = %v, want SEAT_EMPTY", m["code"])
	}

	// 接收方（creator）轮询到待处理申请
	w = doRequest(t, r, "GET", base+"/pending", creator, nil)
	assertStatus(t, w, http.StatusOK)
	pending := parseJSON(t, w)["request"].(map[string]interface{})
	if int64(pending["id"].(float64)) != reqID {
		t.Fatalf("pending id = %v, want %d", pending["id"], reqID)
	}
	if pending["from_nickname"] != "玩家joiner" {
		t.Fatalf("from_nickname = %v, want 玩家joiner", pending["from_nickname"])
	}

	// 非接收方不能越权同意
	w = doRequest(t, r, "POST", fmt.Sprintf("%s/%d/accept", base, reqID), joiner, map[string]string{})
	assertStatus(t, w, http.StatusForbidden)

	// creator 同意 → 双方座位互换
	w = doRequest(t, r, "POST", fmt.Sprintf("%s/%d/accept", base, reqID), creator, map[string]string{})
	assertStatus(t, w, http.StatusOK)

	w = doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d", gameID), creator, nil)
	assertStatus(t, w, http.StatusOK)
	players := parseJSON(t, w)["players"].([]interface{})
	if players[0].(map[string]interface{})["nickname"] != "玩家joiner" {
		t.Fatalf("seat1 = %v, want 玩家joiner", players[0].(map[string]interface{})["nickname"])
	}

	// 处理后不应再有待处理申请
	w = doRequest(t, r, "GET", base+"/pending", creator, nil)
	assertStatus(t, w, http.StatusOK)
	if parseJSON(t, w)["request"] != nil {
		t.Fatal("pending request should be nil after accepted")
	}

	// 发起人（joiner）能查到自己那条申请的最终结果 + 对方昵称
	w = doRequest(t, r, "GET", base+"/pending", joiner, nil)
	assertStatus(t, w, http.StatusOK)
	out := parseJSON(t, w)["outgoing"].(map[string]interface{})
	if out["status"] != "accepted" {
		t.Fatalf("outgoing status = %v, want accepted", out["status"])
	}
	if out["to_nickname"] != "玩家creator" {
		t.Fatalf("to_nickname = %v, want 玩家creator", out["to_nickname"])
	}

	// 拒绝流程：joiner 再申请一次（此时在 1 位），creator 拒绝
	w = doRequest(t, r, "POST", base, joiner, map[string]int{"target_seat": 2})
	assertStatus(t, w, http.StatusCreated)
	reqID2 := int64(parseJSON(t, w)["request"].(map[string]interface{})["id"].(float64))
	w = doRequest(t, r, "POST", fmt.Sprintf("%s/%d/reject", base, reqID2), creator, map[string]string{})
	assertStatus(t, w, http.StatusOK)
	w = doRequest(t, r, "GET", base+"/pending", joiner, nil)
	assertStatus(t, w, http.StatusOK)
	if out2 := parseJSON(t, w)["outgoing"].(map[string]interface{}); out2["status"] != "rejected" {
		t.Fatalf("outgoing status = %v, want rejected", out2["status"])
	}
}

// TestListAdjustmentsVisibleToAllPlayers：流水账单对全桌可见，不能只有台主看得到。
func TestListAdjustmentsVisibleToAllPlayers(t *testing.T) {
	r, _, _ := testSetup(t)
	creator := loginAndAuth(t, r, "creator")
	joiner := loginAndAuth(t, r, "joiner")

	w := doRequest(t, r, "POST", "/api/v1/games", creator, map[string]string{"request_id": "r1"})
	assertStatus(t, w, http.StatusCreated)
	inviteToken := parseJSON(t, w)["invite_token"].(string)
	w = doRequest(t, r, "POST", "/api/v1/games/join", joiner, map[string]string{"invite_token": inviteToken, "request_id": "r2"})
	assertStatus(t, w, http.StatusCreated)
	gameID := int64(parseJSON(t, w)["game_id"].(float64))

	w = doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d/rounds/current", gameID), creator, nil)
	assertStatus(t, w, http.StatusOK)
	roundID := int64(parseJSON(t, w)["round_id"].(float64))

	w = doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d", gameID), creator, nil)
	assertStatus(t, w, http.StatusOK)
	var toPlayerID int64
	for _, p := range parseJSON(t, w)["players"].([]interface{}) {
		pm := p.(map[string]interface{})
		if pm["nickname"] == "玩家joiner" {
			toPlayerID = int64(pm["player_id"].(float64))
		}
	}
	if toPlayerID == 0 {
		t.Fatal("joiner player_id not found")
	}

	// 台主转分给 joiner（auto_accept 立即生效）
	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/rounds/%d/adjustments", gameID, roundID), creator,
		map[string]interface{}{
			"to_player_id":    toPlayerID,
			"adjustment_type": "supplement",
			"amount":          4,
			"auto_accept":     true,
			"request_id":      "adj-auto",
		})
	assertStatus(t, w, http.StatusCreated)

	// 非台主（joiner）必须能看到这笔记录
	w = doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d/adjustments", gameID), joiner, nil)
	assertStatus(t, w, http.StatusOK)
	adjs := parseJSON(t, w)["adjustments"].([]interface{})
	if len(adjs) != 1 {
		t.Fatalf("joiner adjustments = %d, want 1", len(adjs))
	}
}

// TestManualNextRound：手动开下一局（免锁定的局边界由人标记）。
func TestManualNextRound(t *testing.T) {
	r, _, _ := testSetup(t)
	gameID, auth1, _ := createGameAndStart(t, r)

	// 直接手动切局：当前局无旧版提交（纯转分流），允许收尾并开新局
	path := fmt.Sprintf("/api/v1/games/%d/rounds/manual-next", gameID)
	w := doRequest(t, r, "POST", path, auth1, map[string]string{})
	assertStatus(t, w, http.StatusOK)
	m := parseJSON(t, w)
	if m["round_number"].(float64) != 2 {
		t.Fatalf("round_number = %v, want 2", m["round_number"])
	}

	// 新局可正常记账
	w = doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d/rounds/current", gameID), auth1, nil)
	assertStatus(t, w, http.StatusOK)
	roundID := int64(parseJSON(t, w)["round_id"].(float64))

	// 旧版提交未配平时拒绝切局
	_ = doRequest(t, r, "PUT", fmt.Sprintf("/api/v1/games/%d/rounds/%d/submission", gameID, roundID), auth1,
		map[string]interface{}{"score": 10, "request_id": "s1"})
	w = doRequest(t, r, "POST", path, auth1, map[string]string{})
	assertStatus(t, w, http.StatusBadRequest)
	if m := parseJSON(t, w); m["code"] != "ROUND_INCOMPLETE" {
		t.Fatalf("code = %v, want ROUND_INCOMPLETE", m["code"])
	}
}

// createGame4P 开一桌并凑满 4 人（2 人自动开局后第 3/4 人继续凑脚加入）。
func createGame4P(t *testing.T, r *gin.Engine) (int64, []string) {
	t.Helper()
	creator := loginAndAuth(t, r, "p1")
	w := doRequest(t, r, "POST", "/api/v1/games", creator, map[string]string{"request_id": "r1"})
	assertStatus(t, w, http.StatusCreated)
	inviteToken := parseJSON(t, w)["invite_token"].(string)
	auths := []string{creator}
	var gameID int64
	for i, code := range []string{"p2", "p3", "p4"} {
		auth := loginAndAuth(t, r, code)
		w = doRequest(t, r, "POST", "/api/v1/games/join", auth, map[string]string{"invite_token": inviteToken, "request_id": fmt.Sprintf("j%d", i)})
		assertStatus(t, w, http.StatusCreated)
		gameID = int64(parseJSON(t, w)["game_id"].(float64))
		auths = append(auths, auth)
	}
	return gameID, auths
}

// playRound 全员提交分数并锁定该局。
func playRound(t *testing.T, r *gin.Engine, gameID, roundID int64, auths []string, scores []int) {
	t.Helper()
	for i, auth := range auths {
		w := doRequest(t, r, "PUT", fmt.Sprintf("/api/v1/games/%d/rounds/%d/submission", gameID, roundID), auth,
			map[string]interface{}{"score": scores[i], "request_id": fmt.Sprintf("s-%d-%d", roundID, i)})
		assertStatus(t, w, http.StatusOK)
	}
	w := doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/rounds/%d/lock", gameID, roundID), auths[0],
		map[string]string{"request_id": fmt.Sprintf("lock-%d", roundID)})
	assertStatus(t, w, http.StatusOK)
}

func TestRankSettleOnEnd(t *testing.T) {
	r, _, _ := testSetup(t)
	gameID, auths := createGame4P(t, r)

	// 第 1 局：+40 +10 -20 -30
	w := doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d/rounds/current", gameID), auths[0], nil)
	assertStatus(t, w, http.StatusOK)
	round1 := int64(parseJSON(t, w)["round_id"].(float64))
	playRound(t, r, gameID, round1, auths, []int{40, 10, -20, -30})

	// 第 2 局：+5 +5 -25 +15
	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/rounds", gameID), auths[0], map[string]string{"request_id": "n2"})
	assertStatus(t, w, http.StatusCreated)
	w = doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d/rounds/current", gameID), auths[0], nil)
	round2 := int64(parseJSON(t, w)["round_id"].(float64))
	playRound(t, r, gameID, round2, auths, []int{5, 5, -25, 15})

	// 散台（总分：p1 +45 胜 / p2 +15 平 / p4 -15 平 / p3 -45 负）
	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/end", gameID), auths[0], map[string]string{"request_id": "e1"})
	assertStatus(t, w, http.StatusOK)

	// p1：胜 +1 星
	w = doRequest(t, r, "GET", "/api/v1/rank/me", auths[0], nil)
	assertStatus(t, w, http.StatusOK)
	m := parseJSON(t, w)
	if m["stars"].(float64) != 1 || m["wins"].(float64) != 1 || m["streak"].(float64) != 1 || m["points"].(float64) != 45 {
		t.Fatalf("p1 rank = %v", m)
	}
	if tier := m["tier"].(map[string]interface{}); tier["tier_short"] != "九品" || tier["stars_in_tier"].(float64) != 1 {
		t.Fatalf("p1 tier = %v", tier)
	}

	// p3：末位 -1 星，但九品 0 星保底不掉
	w = doRequest(t, r, "GET", "/api/v1/rank/me", auths[2], nil)
	m = parseJSON(t, w)
	if m["stars"].(float64) != 0 || m["losses"].(float64) != 1 || m["points"].(float64) != -45 {
		t.Fatalf("p3 rank = %v", m)
	}

	// p4：第三名（-15）按名次为平，不扣星
	w = doRequest(t, r, "GET", "/api/v1/rank/me", auths[3], nil)
	m = parseJSON(t, w)
	if m["stars"].(float64) != 0 || m["draws"].(float64) != 1 {
		t.Fatalf("p4 rank = %v, want draw no star change", m)
	}

	// 历史详情带排位变动
	w = doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d/history", gameID), auths[0], nil)
	assertStatus(t, w, http.StatusOK)
	m = parseJSON(t, w)
	if rc, ok := m["rank_changes"].([]interface{}); !ok || len(rc) != 4 {
		t.Fatalf("rank_changes = %v, want 4 entries", m["rank_changes"])
	}
	if players, ok := m["players"].([]interface{}); !ok || len(players) != 4 {
		t.Fatalf("players = %v, want 4", m["players"])
	}

	// 记录页标签筛选：p1 能按「胜」筛到，p3 按「负」筛到；日期筛选命中
	w = doRequest(t, r, "GET", "/api/v1/games/history?result=win", auths[0], nil)
	assertStatus(t, w, http.StatusOK)
	m = parseJSON(t, w)
	if m["total"].(float64) != 1 {
		t.Fatalf("history result=win total = %v", m["total"])
	}
	g := m["games"].([]interface{})[0].(map[string]interface{})
	if g["my_score"].(float64) != 45 || g["my_rank"].(float64) != 1 || g["is_ranked"] != true {
		t.Fatalf("history item = %v", g)
	}

	w = doRequest(t, r, "GET", "/api/v1/games/history?result=lose", auths[2], nil)
	m = parseJSON(t, w)
	if m["total"].(float64) != 1 {
		t.Fatalf("history result=lose total = %v", m["total"])
	}

	w = doRequest(t, r, "GET", "/api/v1/games/history?days=7", auths[0], nil)
	m = parseJSON(t, w)
	if m["total"].(float64) != 1 {
		t.Fatalf("history days=7 total = %v", m["total"])
	}

	// 分页字段
	w = doRequest(t, r, "GET", "/api/v1/games/history?page=1&page_size=20", auths[0], nil)
	m = parseJSON(t, w)
	if _, ok := m["has_more"]; !ok {
		t.Fatal("missing has_more")
	}
}

func TestRoomExclusivity(t *testing.T) {
	r, _, _ := testSetup(t)
	creator := loginAndAuth(t, r, "ex-a")
	joiner := loginAndAuth(t, r, "ex-b")
	other := loginAndAuth(t, r, "ex-c")

	// creator 开台，joiner 加入（2 人自动开局）
	w := doRequest(t, r, "POST", "/api/v1/games", creator, map[string]string{"request_id": "ex-1"})
	assertStatus(t, w, http.StatusCreated)
	game1 := int64(parseJSON(t, w)["game_id"].(float64))
	invite1 := parseJSON(t, w)["invite_token"].(string)
	w = doRequest(t, r, "POST", "/api/v1/games/join", joiner, map[string]string{"invite_token": invite1, "request_id": "ex-2"})
	assertStatus(t, w, http.StatusCreated)

	// creator 已有进行中的牌台：再开一张 → 409 并返回现有牌台 ID
	w = doRequest(t, r, "POST", "/api/v1/games", creator, map[string]string{"request_id": "ex-3"})
	assertStatus(t, w, http.StatusConflict)
	m := parseJSON(t, w)
	if m["code"] != "ALREADY_IN_GAME" || int64(m["game_id"].(float64)) != game1 {
		t.Fatalf("create conflict = %v", m)
	}

	// other 开第二张台，joiner 想加入 → 409
	w = doRequest(t, r, "POST", "/api/v1/games", other, map[string]string{"request_id": "ex-4"})
	assertStatus(t, w, http.StatusCreated)
	invite2 := parseJSON(t, w)["invite_token"].(string)
	w = doRequest(t, r, "POST", "/api/v1/games/join", joiner, map[string]string{"invite_token": invite2, "request_id": "ex-5"})
	assertStatus(t, w, http.StatusConflict)
	if m := parseJSON(t, w); m["code"] != "ALREADY_IN_GAME" {
		t.Fatalf("join conflict = %v", m)
	}

	// creator 取消自己的台后释放，可再开
	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/cancel", game1), creator, map[string]string{"request_id": "ex-6"})
	assertStatus(t, w, http.StatusOK)
	w = doRequest(t, r, "POST", "/api/v1/games", creator, map[string]string{"request_id": "ex-7"})
	assertStatus(t, w, http.StatusCreated)
}

func TestStartGameNotEnoughPlayers(t *testing.T) {
	r, _, _ := testSetup(t)
	auth := loginAndAuth(t, r, "creator")

	w := doRequest(t, r, "POST", "/api/v1/games", auth, map[string]string{"request_id": "r1"})
	m := parseJSON(t, w)
	gameID := int64(m["game_id"].(float64))

	// Try to start with 1 player
	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/start", gameID), auth, map[string]string{"request_id": "r2"})
	assertStatus(t, w, http.StatusBadRequest)
}

func TestCancelGame(t *testing.T) {
	r, _, _ := testSetup(t)
	auth := loginAndAuth(t, r, "creator")

	w := doRequest(t, r, "POST", "/api/v1/games", auth, map[string]string{"request_id": "r1"})
	m := parseJSON(t, w)
	gameID := int64(m["game_id"].(float64))

	// Cancel
	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/cancel", gameID), auth, map[string]string{"request_id": "r2"})
	assertStatus(t, w, http.StatusOK)
}

func TestCancelActiveGameWithoutScores(t *testing.T) {
	r, _, _ := testSetup(t)
	gameID, auth1, _ := createGameAndStart(t, r)

	// 已自动开局但还没有入账的局：仍可取消开台
	w := doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/cancel", gameID), auth1, map[string]string{"request_id": "r2"})
	assertStatus(t, w, http.StatusOK)
}

func TestCancelWithScoresRejected(t *testing.T) {
	r, _, _ := testSetup(t)
	gameID, auth1, auth2 := createGameAndStart(t, r)

	w := doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d/rounds/current", gameID), auth1, nil)
	assertStatus(t, w, http.StatusOK)
	roundID := int64(parseJSON(t, w)["round_id"].(float64))

	// 双方提交并锁定一局，产生记分记录
	w = doRequest(t, r, "PUT", fmt.Sprintf("/api/v1/games/%d/rounds/%d/submission", gameID, roundID), auth1,
		map[string]interface{}{"score": 16, "request_id": "sub1"})
	assertStatus(t, w, http.StatusOK)
	w = doRequest(t, r, "PUT", fmt.Sprintf("/api/v1/games/%d/rounds/%d/submission", gameID, roundID), auth2,
		map[string]interface{}{"score": -16, "request_id": "sub2"})
	assertStatus(t, w, http.StatusOK)
	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/rounds/%d/lock", gameID, roundID), auth1,
		map[string]string{"request_id": "lock1"})
	assertStatus(t, w, http.StatusOK)

	// 有记分记录：不能取消，只能散台结算
	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/cancel", gameID), auth1, map[string]string{"request_id": "r2"})
	assertStatus(t, w, http.StatusBadRequest)
	if m := parseJSON(t, w); m["code"] != "GAME_HAS_SCORES" {
		t.Fatalf("code = %v, want GAME_HAS_SCORES", m["code"])
	}
}

// ---------------------------------------------------------------------------
// Round and submission tests
// ---------------------------------------------------------------------------

func TestFullRoundFlow(t *testing.T) {
	r, _, _ := testSetup(t)
	gameID, auth1, auth2 := createGameAndStart(t, r)

	// Get current round
	w := doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d/rounds/current", gameID), auth1, nil)
	assertStatus(t, w, http.StatusOK)
	m := parseJSON(t, w)
	if m["round_number"].(float64) != 1 {
		t.Fatalf("round_number = %v, want 1", m["round_number"])
	}
	if m["status"] != "open" {
		t.Fatalf("status = %v, want open", m["status"])
	}

	roundID := int64(m["round_id"].(float64))

	// Player 1 submits +16
	w = doRequest(t, r, "PUT", fmt.Sprintf("/api/v1/games/%d/rounds/%d/submission", gameID, roundID), auth1,
		map[string]interface{}{"score": 16, "request_id": "sub1"})
	assertStatus(t, w, http.StatusOK)
	m = parseJSON(t, w)
	if m["my_submitted"] != true {
		t.Fatal("my_submitted should be true")
	}
	if m["submitted_count"].(float64) != 1 {
		t.Fatalf("submitted_count = %v, want 1", m["submitted_count"])
	}

	// Player 2 submits -16
	w = doRequest(t, r, "PUT", fmt.Sprintf("/api/v1/games/%d/rounds/%d/submission", gameID, roundID), auth2,
		map[string]interface{}{"score": -16, "request_id": "sub2"})
	assertStatus(t, w, http.StatusOK)
	m = parseJSON(t, w)
	if m["round_status"] != "review" {
		t.Fatalf("round_status = %v, want review", m["round_status"])
	}
	if m["submitted_count"].(float64) != 2 {
		t.Fatalf("submitted_count = %v, want 2", m["submitted_count"])
	}
	if int64(m["score_sum"].(float64)) != 0 {
		t.Fatalf("score_sum = %v, want 0", m["score_sum"])
	}

	// Lock round
	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/rounds/%d/lock", gameID, roundID), auth1,
		map[string]string{"request_id": "lock1"})
	assertStatus(t, w, http.StatusOK)
	m = parseJSON(t, w)
	if m["status"] != "ready_for_next" {
		t.Fatalf("status = %v, want ready_for_next", m["status"])
	}
	if m["completed_rounds"].(float64) != 1 {
		t.Fatalf("completed_rounds = %v, want 1", m["completed_rounds"])
	}

	// Create next round
	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/rounds", gameID), auth1,
		map[string]string{"request_id": "next1"})
	assertStatus(t, w, http.StatusCreated)
	m = parseJSON(t, w)
	if m["round_number"].(float64) != 2 {
		t.Fatalf("round_number = %v, want 2", m["round_number"])
	}
}

func TestZeroSumFail(t *testing.T) {
	r, _, _ := testSetup(t)
	gameID, auth1, auth2 := createGameAndStart(t, r)

	// Get round
	w := doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d/rounds/current", gameID), auth1, nil)
	m := parseJSON(t, w)
	roundID := int64(m["round_id"].(float64))

	// Submit non-zero-sum
	w = doRequest(t, r, "PUT", fmt.Sprintf("/api/v1/games/%d/rounds/%d/submission", gameID, roundID), auth1,
		map[string]interface{}{"score": 16, "request_id": "s1"})
	assertStatus(t, w, http.StatusOK)

	w = doRequest(t, r, "PUT", fmt.Sprintf("/api/v1/games/%d/rounds/%d/submission", gameID, roundID), auth2,
		map[string]interface{}{"score": -8, "request_id": "s2"})
	assertStatus(t, w, http.StatusOK)

	// Try to lock — should fail (sum = 8)
	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/rounds/%d/lock", gameID, roundID), auth1,
		map[string]string{"request_id": "l1"})
	assertStatus(t, w, http.StatusBadRequest)
	m = parseJSON(t, w)
	if m["code"] != "ZERO_SUM_FAILED" {
		t.Fatalf("code = %v, want ZERO_SUM_FAILED", m["code"])
	}
}

func TestEndGameWithIncompleteRound(t *testing.T) {
	r, _, _ := testSetup(t)
	gameID, auth1, _ := createGameAndStart(t, r)

	// Try to end without completing round
	w := doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/end", gameID), auth1, map[string]string{"request_id": "e1"})
	assertStatus(t, w, http.StatusBadRequest)
}

func TestEndGameSuccess(t *testing.T) {
	r, _, _ := testSetup(t)
	gameID, auth1, auth2 := createGameAndStart(t, r)

	// Get round
	w := doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d/rounds/current", gameID), auth1, nil)
	m := parseJSON(t, w)
	roundID := int64(m["round_id"].(float64))

	// Both submit zero-sum
	_ = doRequest(t, r, "PUT", fmt.Sprintf("/api/v1/games/%d/rounds/%d/submission", gameID, roundID), auth1,
		map[string]interface{}{"score": 10, "request_id": "s1"})
	_ = doRequest(t, r, "PUT", fmt.Sprintf("/api/v1/games/%d/rounds/%d/submission", gameID, roundID), auth2,
		map[string]interface{}{"score": -10, "request_id": "s2"})

	// Lock
	_ = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/rounds/%d/lock", gameID, roundID), auth1,
		map[string]string{"request_id": "l1"})

	// End game
	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/end", gameID), auth1, map[string]string{"request_id": "e1"})
	assertStatus(t, w, http.StatusOK)
}

// ---------------------------------------------------------------------------
// Settlement test
// ---------------------------------------------------------------------------

func TestSettlement(t *testing.T) {
	r, _, _ := testSetup(t)
	gameID, auth1, auth2 := createGameAndStart(t, r)

	// Round 1: +20 / -20
	w := doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d/rounds/current", gameID), auth1, nil)
	m := parseJSON(t, w)
	roundID := int64(m["round_id"].(float64))

	_ = doRequest(t, r, "PUT", fmt.Sprintf("/api/v1/games/%d/rounds/%d/submission", gameID, roundID), auth1,
		map[string]interface{}{"score": 20, "request_id": "s1"})
	_ = doRequest(t, r, "PUT", fmt.Sprintf("/api/v1/games/%d/rounds/%d/submission", gameID, roundID), auth2,
		map[string]interface{}{"score": -20, "request_id": "s2"})
	_ = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/rounds/%d/lock", gameID, roundID), auth1,
		map[string]string{"request_id": "l1"})

	// End game
	_ = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/end", gameID), auth1, map[string]string{"request_id": "e1"})

	// Get settlement
	w = doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d/settlement", gameID), auth1, nil)
	assertStatus(t, w, http.StatusOK)
	m = parseJSON(t, w)
	if m["completed_rounds"].(float64) != 1 {
		t.Fatalf("completed_rounds = %v, want 1", m["completed_rounds"])
	}
	players := m["players"].([]interface{})
	if len(players) != 2 {
		t.Fatalf("players count = %d, want 2", len(players))
	}
	p0 := players[0].(map[string]interface{})
	if p0["rank"].(float64) != 1 {
		t.Fatalf("rank = %v, want 1", p0["rank"])
	}
}

// ---------------------------------------------------------------------------
// Adjustment test
// ---------------------------------------------------------------------------

func TestAdjustmentFlow(t *testing.T) {
	r, _, _ := testSetup(t)
	gameID, auth1, auth2 := createGameAndStart(t, r)

	// Round 1: +10 / -10
	w := doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d/rounds/current", gameID), auth1, nil)
	m := parseJSON(t, w)
	roundID := int64(m["round_id"].(float64))

	_ = doRequest(t, r, "PUT", fmt.Sprintf("/api/v1/games/%d/rounds/%d/submission", gameID, roundID), auth1,
		map[string]interface{}{"score": 10, "request_id": "s1"})
	_ = doRequest(t, r, "PUT", fmt.Sprintf("/api/v1/games/%d/rounds/%d/submission", gameID, roundID), auth2,
		map[string]interface{}{"score": -10, "request_id": "s2"})
	_ = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/rounds/%d/lock", gameID, roundID), auth1,
		map[string]string{"request_id": "l1"})

	// End game
	_ = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/end", gameID), auth1, map[string]string{"request_id": "e1"})

	// Get game players to find player IDs
	w = doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d", gameID), auth1, nil)
	m = parseJSON(t, w)
	players := m["players"].([]interface{})
	var toPlayerID int64
	for _, p := range players {
		pMap := p.(map[string]interface{})
		if pMap["role"] == "player" {
			toPlayerID = int64(pMap["player_id"].(float64))
		}
	}

	// Create adjustment
	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/rounds/%d/adjustments", gameID, roundID), auth1,
		map[string]interface{}{
			"to_player_id":    toPlayerID,
			"adjustment_type": "supplement",
			"amount":          5,
			"reason":          "少记了5分",
			"request_id":      "adj1",
		})
	assertStatus(t, w, http.StatusCreated)
	m = parseJSON(t, w)
	adj := m["adjustment"].(map[string]interface{})
	adjustmentID := int64(adj["id"].(float64))

	// Accept adjustment (by to_player, which is auth2)
	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/adjustments/%d/accept", gameID, adjustmentID), auth2, map[string]string{"request_id": "acc1"})
	assertStatus(t, w, http.StatusOK)

	// Check settlement reflects adjustment
	w = doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d/settlement", gameID), auth1, nil)
	m = parseJSON(t, w)
	if m["adjustment_count"].(float64) != 1 {
		t.Fatalf("adjustment_count = %v, want 1", m["adjustment_count"])
	}
}

// ---------------------------------------------------------------------------
// Open phase privacy test (PRD §2.3 rule 5)
// ---------------------------------------------------------------------------

func TestOpenPhaseHidesOtherScores(t *testing.T) {
	r, _, _ := testSetup(t)
	gameID, auth1, auth2 := createGameAndStart(t, r)

	// Player 1 submits
	w := doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d/rounds/current", gameID), auth1, nil)
	m := parseJSON(t, w)
	roundID := int64(m["round_id"].(float64))

	_ = doRequest(t, r, "PUT", fmt.Sprintf("/api/v1/games/%d/rounds/%d/submission", gameID, roundID), auth1,
		map[string]interface{}{"score": 16, "request_id": "s1"})

	// Player 2 checks current round — should NOT see player 1's score
	w = doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d/rounds/current", gameID), auth2, nil)
	assertStatus(t, w, http.StatusOK)
	m = parseJSON(t, w)
	subs := m["submissions"].([]interface{})
	for _, s := range subs {
		sMap := s.(map[string]interface{})
		// Only see status, not score of others
		if !sMap["submitted"].(bool) {
			// Not submitted = score should be 0
			if sMap["score"].(float64) != 0 {
				t.Fatal("unsubmitted player should show score 0")
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Idempotent create next round test (PRD §2.2 rule 8)
// ---------------------------------------------------------------------------

func TestIdempotentNextRound(t *testing.T) {
	r, _, _ := testSetup(t)
	gameID, auth1, auth2 := createGameAndStart(t, r)

	w := doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d/rounds/current", gameID), auth1, nil)
	m := parseJSON(t, w)
	roundID := int64(m["round_id"].(float64))

	_ = doRequest(t, r, "PUT", fmt.Sprintf("/api/v1/games/%d/rounds/%d/submission", gameID, roundID), auth1,
		map[string]interface{}{"score": 5, "request_id": "s1"})
	_ = doRequest(t, r, "PUT", fmt.Sprintf("/api/v1/games/%d/rounds/%d/submission", gameID, roundID), auth2,
		map[string]interface{}{"score": -5, "request_id": "s2"})
	_ = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/rounds/%d/lock", gameID, roundID), auth1,
		map[string]string{"request_id": "l1"})

	// Create next round twice
	w1 := doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/rounds", gameID), auth1,
		map[string]string{"request_id": "n1"})
	assertStatus(t, w1, http.StatusCreated)
	m1 := parseJSON(t, w1)
	if m1["round_number"].(float64) != 2 {
		t.Fatalf("first: round_number = %v, want 2", m1["round_number"])
	}

	// Second request should also return round 2 (idempotent)
	w2 := doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/rounds", gameID), auth2,
		map[string]string{"request_id": "n2"})
	m2 := parseJSON(t, w2)
	if m2["round_number"] == nil {
		t.Fatalf("second: round_number is nil, body: %s", w2.Body.String())
	}
	if m2["round_number"].(float64) != 2 {
		t.Fatalf("second: round_number = %v, want 2", m2["round_number"])
	}
}

// ---------------------------------------------------------------------------
// Auto-start / hide / leaderboard tests
// ---------------------------------------------------------------------------

// TestJoinAutoStart verifies a forming game auto-activates with round 1 as
// soon as the second player joins (房间页无需开始记分按钮).
func TestJoinAutoStart(t *testing.T) {
	r, _, _ := testSetup(t)

	creator := loginAndAuth(t, r, "autostart-c")
	// Empty name must yield the auto-generated default table name
	w := doRequest(t, r, "POST", "/api/v1/games", creator,
		map[string]string{"name": "", "request_id": "as-create"})
	assertStatus(t, w, http.StatusCreated)
	m := parseJSON(t, w)
	gameID := int64(m["game_id"].(float64))
	inviteToken := m["invite_token"].(string)
	if m["name"].(string) == "" {
		t.Fatal("game name should not be empty")
	}

	// Solo room: still forming, no round yet
	w = doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d", gameID), creator, nil)
	assertStatus(t, w, http.StatusOK)
	if m = parseJSON(t, w); m["status"] != "forming" {
		t.Fatalf("solo room status = %v, want forming", m["status"])
	}

	// Second player joins → game auto-starts
	joiner := loginAndAuth(t, r, "autostart-j")
	w = doRequest(t, r, "POST", "/api/v1/games/join", joiner,
		map[string]string{"invite_token": inviteToken, "request_id": "as-join"})
	assertStatus(t, w, http.StatusCreated)
	m = parseJSON(t, w)
	if m["status"] != "active" {
		t.Fatalf("after 2nd join status = %v, want active", m["status"])
	}

	// Round 1 must exist for both players
	w = doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d/rounds/current", gameID), creator, nil)
	assertStatus(t, w, http.StatusOK)
	m = parseJSON(t, w)
	if m["current_round_number"] != float64(1) {
		t.Fatalf("current_round_number = %v, want 1", m["current_round_number"])
	}

	// Third/Fourth join while active: 开局后未满 4 人仍可继续凑脚（排位需要 4 人局）
	third := loginAndAuth(t, r, "autostart-t")
	w = doRequest(t, r, "POST", "/api/v1/games/join", third,
		map[string]string{"invite_token": inviteToken, "request_id": "as-join3"})
	assertStatus(t, w, http.StatusCreated)

	fourth := loginAndAuth(t, r, "autostart-f")
	w = doRequest(t, r, "POST", "/api/v1/games/join", fourth,
		map[string]string{"invite_token": inviteToken, "request_id": "as-join4"})
	assertStatus(t, w, http.StatusCreated)

	// Fifth join: 满 4 人后锁定，不再接受加入
	fifth := loginAndAuth(t, r, "autostart-5")
	w = doRequest(t, r, "POST", "/api/v1/games/join", fifth,
		map[string]string{"invite_token": inviteToken, "request_id": "as-join5"})
	assertStatus(t, w, http.StatusBadRequest)
}

// TestHideGame verifies hiding an ended/cancelled game from one's own history.
func TestHideGame(t *testing.T) {
	r, _, _ := testSetup(t)

	// Use a full game (start + end) to create a history entry,
	// since cancel now physically deletes games without scores.
	gameID, auth1, auth2 := createGameAndStart(t, r)

	// Lock a round with scores so the game has data, then end it
	w := doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d/rounds/current", gameID), auth1, nil)
	assertStatus(t, w, http.StatusOK)
	roundID := int64(parseJSON(t, w)["round_id"].(float64))

	w = doRequest(t, r, "PUT", fmt.Sprintf("/api/v1/games/%d/rounds/%d/submission", gameID, roundID), auth1,
		map[string]interface{}{"score": 8, "request_id": "h-sub1"})
	assertStatus(t, w, http.StatusOK)
	w = doRequest(t, r, "PUT", fmt.Sprintf("/api/v1/games/%d/rounds/%d/submission", gameID, roundID), auth2,
		map[string]interface{}{"score": -8, "request_id": "h-sub2"})
	assertStatus(t, w, http.StatusOK)
	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/rounds/%d/lock", gameID, roundID), auth1,
		map[string]string{"request_id": "h-lock"})
	assertStatus(t, w, http.StatusOK)

	// End the game → lands in history list
	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/end", gameID), auth1, map[string]string{"request_id": "h-end"})
	assertStatus(t, w, http.StatusOK)

	w = doRequest(t, r, "GET", "/api/v1/games/history", auth1, nil)
	assertStatus(t, w, http.StatusOK)
	if m := parseJSON(t, w); m["total"] != float64(1) {
		t.Fatalf("history total = %v, want 1", m["total"])
	}

	// Hide it
	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/hide", gameID), auth1,
		map[string]string{"request_id": "h-hide"})
	assertStatus(t, w, http.StatusOK)

	// Idempotent hide
	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/hide", gameID), auth1,
		map[string]string{"request_id": "h-hide2"})
	assertStatus(t, w, http.StatusOK)

	w = doRequest(t, r, "GET", "/api/v1/games/history", auth1, nil)
	assertStatus(t, w, http.StatusOK)
	if m := parseJSON(t, w); m["total"] != float64(0) {
		t.Fatalf("history total after hide = %v, want 0", m["total"])
	}
}

// TestLeaderboardAndUserStats runs a full zero-sum game then checks the
// leaderboard and personal stats endpoints.
func TestLeaderboardAndUserStats(t *testing.T) {
	r, _, _ := testSetup(t)

	auth1 := loginAndAuth(t, r, "lb-a")
	auth2 := loginAndAuth(t, r, "lb-b")

	// 3 场满 4 人排位局，auth1 每场第一（+16）
	for gi := 0; gi < 3; gi++ {
		w := doRequest(t, r, "POST", "/api/v1/games", auth1, map[string]string{"request_id": fmt.Sprintf("lb-create-%d", gi)})
		assertStatus(t, w, http.StatusCreated)
		m := parseJSON(t, w)
		gameID := int64(m["game_id"].(float64))
		inviteToken := m["invite_token"].(string)

		w = doRequest(t, r, "POST", "/api/v1/games/join", auth2,
			map[string]string{"invite_token": inviteToken, "request_id": fmt.Sprintf("lb-join-%d", gi)})
		assertStatus(t, w, http.StatusCreated)

		// 再补两名玩家凑满 4 人（积分榜只计满 4 人局）
		auths := []string{auth1, auth2}
		for pi, code := range []string{"lb-c", "lb-d"} {
			ca := loginAndAuth(t, r, code)
			w = doRequest(t, r, "POST", "/api/v1/games/join", ca,
				map[string]string{"invite_token": inviteToken, "request_id": fmt.Sprintf("lb-join%d-%d", pi, gi)})
			assertStatus(t, w, http.StatusCreated)
			auths = append(auths, ca)
		}

		w = doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d/rounds/current", gameID), auths[0], nil)
		assertStatus(t, w, http.StatusOK)
		roundID := int64(parseJSON(t, w)["round_id"].(float64))
		playRound(t, r, gameID, roundID, auths, []int{16, 5, -8, -13})
		w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/end", gameID), auths[0],
			map[string]string{"request_id": fmt.Sprintf("lb-end-%d", gi)})
		assertStatus(t, w, http.StatusOK)
	}

	// 2 人局不计入积分榜（积分榜与段位榜同口径：仅满 4 人局）
	w := doRequest(t, r, "POST", "/api/v1/games", auth1, map[string]string{"request_id": "lb-2p-create"})
	assertStatus(t, w, http.StatusCreated)
	m := parseJSON(t, w)
	gameID := int64(m["game_id"].(float64))
	inviteToken := m["invite_token"].(string)
	w = doRequest(t, r, "POST", "/api/v1/games/join", auth2,
		map[string]string{"invite_token": inviteToken, "request_id": "lb-2p-join"})
	assertStatus(t, w, http.StatusCreated)
	w = doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d/rounds/current", gameID), auth1, nil)
	m = parseJSON(t, w)
	roundID := int64(m["round_id"].(float64))
	w = doRequest(t, r, "PUT", fmt.Sprintf("/api/v1/games/%d/rounds/%d/submission", gameID, roundID), auth1,
		map[string]interface{}{"score": 16, "request_id": "lb-2p-s1"})
	assertStatus(t, w, http.StatusOK)
	w = doRequest(t, r, "PUT", fmt.Sprintf("/api/v1/games/%d/rounds/%d/submission", gameID, roundID), auth2,
		map[string]interface{}{"score": -16, "request_id": "lb-2p-s2"})
	assertStatus(t, w, http.StatusOK)
	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/rounds/%d/lock", gameID, roundID), auth1,
		map[string]string{"request_id": "lb-2p-lock"})
	assertStatus(t, w, http.StatusOK)
	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/end", gameID), auth1,
		map[string]string{"request_id": "lb-2p-end"})
	assertStatus(t, w, http.StatusOK)

	// Leaderboard：只统计 4 人局 → 4 名玩家各 3 场
	w = doRequest(t, r, "GET", "/api/v1/leaderboard", auth1, nil)
	assertStatus(t, w, http.StatusOK)
	m = parseJSON(t, w)
	lb := m["leaderboard"].([]interface{})
	if len(lb) != 4 {
		t.Fatalf("leaderboard size = %d, want 4 (only 4-player games count)", len(lb))
	}
	first := lb[0].(map[string]interface{})
	if first["user_id"] == nil || first["win_rate"] != float64(100) {
		t.Fatalf("leaderboard[0] = %v, want the winner with 100%% win rate", first)
	}
	if first["best_streak"].(float64) != 3 {
		t.Fatalf("leaderboard[0] best_streak = %v, want 3", first["best_streak"])
	}
	if first["best_score"].(float64) != 16 {
		t.Fatalf("leaderboard[0] best_score = %v, want 16", first["best_score"])
	}
	tags := first["tags"].([]interface{})
	// v1.3 规则：连胜王需最高连胜 ≥ 4，本场景 3 连胜不应点亮
	for _, tag := range tags {
		if tag == "连胜王" {
			t.Fatalf("leaderboard[0] tags = %v, 连胜王 requires best_streak >= 4 (got %v)", tags, first["best_streak"])
		}
	}

	// 时间筛选：近 7 天命中
	w = doRequest(t, r, "GET", "/api/v1/leaderboard?days=7", auth1, nil)
	m = parseJSON(t, w)
	if len(m["leaderboard"].([]interface{})) != 4 {
		t.Fatalf("leaderboard days=7 size = %v, want 4", m["leaderboard"])
	}
}
