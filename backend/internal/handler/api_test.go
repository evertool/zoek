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

			auth.POST("/games/:game_id/rounds", roundH.CreateNextRound)
			auth.GET("/games/:game_id/rounds/current", roundH.GetCurrentRound)
			auth.PUT("/games/:game_id/rounds/:round_id/submission", roundH.SubmitScore)
			auth.POST("/games/:game_id/rounds/:round_id/lock", roundH.LockRound)
			auth.POST("/games/:game_id/rounds/:round_id/next", roundH.CreateNextRound)
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

	// Save profile (nickname + base64 data-URL avatar)
	w = doRequest(t, r, "PUT", "/api/v1/user/profile", token,
		map[string]string{"nickname": "阿强", "avatar_url": "data:image/jpeg;base64,AAAA"})
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
	if m["avatar_url"] != "data:image/jpeg;base64,AAAA" {
		t.Fatalf("re-login avatar_url = %v, want saved data URL", m["avatar_url"])
	}
}

// TestProfileTempAvatarRejected verifies that a WeChat chooseAvatar temporary
// path (http://tmp/, wxfile://) is not accepted as a persistent avatar: it
// dies after app restart, so the user must still complete their profile.
func TestProfileTempAvatarRejected(t *testing.T) {
	r, _, _ := testSetup(t)

	w := doRequest(t, r, "POST", "/api/v1/auth/login", "", map[string]string{"code": "tmpav"})
	assertStatus(t, w, http.StatusOK)
	m := parseJSON(t, w)
	token := "Bearer " + m["token"].(string)

	// Legacy client behaviour: saving the raw temporary avatar path
	w = doRequest(t, r, "PUT", "/api/v1/user/profile", token,
		map[string]string{"nickname": "阿明", "avatar_url": "http://tmp/R1S_deadbeef"})
	assertStatus(t, w, http.StatusOK)
	m = parseJSON(t, w)
	if m["need_profile"] != true {
		t.Fatalf("after saving tmp avatar need_profile = %v, want true", m["need_profile"])
	}
	if m["avatar_url"] != "" {
		t.Fatalf("tmp avatar must not be stored, got %v", m["avatar_url"])
	}

	// Re-login: avatar is gone after restart, profile still incomplete
	w = doRequest(t, r, "POST", "/api/v1/auth/login", "", map[string]string{"code": "tmpav"})
	assertStatus(t, w, http.StatusOK)
	m = parseJSON(t, w)
	if m["need_profile"] != true {
		t.Fatalf("re-login with only tmp avatar need_profile = %v, want true", m["need_profile"])
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

	// Third join while active: QR still works for invite, but join is blocked (members implicit lock via status != forming)
	third := loginAndAuth(t, r, "autostart-t")
	w = doRequest(t, r, "POST", "/api/v1/games/join", third,
		map[string]string{"invite_token": inviteToken, "request_id": "as-join3"})
	assertStatus(t, w, http.StatusBadRequest)
}

// TestHideGame verifies hiding an ended/cancelled game from one's own history.
func TestHideGame(t *testing.T) {
	r, _, _ := testSetup(t)

	auth := loginAndAuth(t, r, "hider")
	w := doRequest(t, r, "POST", "/api/v1/games", auth, map[string]string{"request_id": "h-create"})
	assertStatus(t, w, http.StatusCreated)
	gameID := int64(parseJSON(t, w)["game_id"].(float64))

	// Cancel the forming game → lands in history list
	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/cancel", gameID), auth,
		map[string]string{"request_id": "h-cancel"})
	assertStatus(t, w, http.StatusOK)

	w = doRequest(t, r, "GET", "/api/v1/games/history", auth, nil)
	assertStatus(t, w, http.StatusOK)
	if m := parseJSON(t, w); m["total"] != float64(1) {
		t.Fatalf("history total = %v, want 1", m["total"])
	}

	// Hide it
	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/hide", gameID), auth,
		map[string]string{"request_id": "h-hide"})
	assertStatus(t, w, http.StatusOK)

	// Idempotent hide
	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/hide", gameID), auth,
		map[string]string{"request_id": "h-hide2"})
	assertStatus(t, w, http.StatusOK)

	w = doRequest(t, r, "GET", "/api/v1/games/history", auth, nil)
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
	w := doRequest(t, r, "POST", "/api/v1/games", auth1, map[string]string{"request_id": "lb-create"})
	assertStatus(t, w, http.StatusCreated)
	m := parseJSON(t, w)
	gameID := int64(m["game_id"].(float64))
	inviteToken := m["invite_token"].(string)

	auth2 := loginAndAuth(t, r, "lb-b")
	w = doRequest(t, r, "POST", "/api/v1/games/join", auth2,
		map[string]string{"invite_token": inviteToken, "request_id": "lb-join"})
	assertStatus(t, w, http.StatusCreated)

	// Round 1 exists via auto-start
	w = doRequest(t, r, "GET", fmt.Sprintf("/api/v1/games/%d/rounds/current", gameID), auth1, nil)
	m = parseJSON(t, w)
	roundID := int64(m["round_id"].(float64))

	// Both submit +16 / -16, then lock
	w = doRequest(t, r, "PUT", fmt.Sprintf("/api/v1/games/%d/rounds/%d/submission", gameID, roundID), auth1,
		map[string]interface{}{"score": 16, "request_id": "lb-s1"})
	assertStatus(t, w, http.StatusOK)
	w = doRequest(t, r, "PUT", fmt.Sprintf("/api/v1/games/%d/rounds/%d/submission", gameID, roundID), auth2,
		map[string]interface{}{"score": -16, "request_id": "lb-s2"})
	assertStatus(t, w, http.StatusOK)
	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/rounds/%d/lock", gameID, roundID), auth1,
		map[string]string{"request_id": "lb-lock"})
	assertStatus(t, w, http.StatusOK)

	// End the game
	w = doRequest(t, r, "POST", fmt.Sprintf("/api/v1/games/%d/end", gameID), auth1,
		map[string]string{"request_id": "lb-end"})
	assertStatus(t, w, http.StatusOK)

	// Personal stats
	w = doRequest(t, r, "GET", "/api/v1/user/stats", auth1, nil)
	assertStatus(t, w, http.StatusOK)
	m = parseJSON(t, w)
	if m["games"] != float64(1) {
		t.Fatalf("stats games = %v, want 1", m["games"])
	}
	if m["wins"] != float64(1) { // +16 vs -16 → rank 1
		t.Fatalf("stats wins = %v, want 1", m["wins"])
	}
	trend, ok := m["trend"].([]interface{})
	if !ok || len(trend) != 1 {
		t.Fatalf("stats trend = %v, want 1 point", m["trend"])
	}

	// Leaderboard contains both players
	w = doRequest(t, r, "GET", "/api/v1/leaderboard", auth1, nil)
	assertStatus(t, w, http.StatusOK)
	m = parseJSON(t, w)
	lb := m["leaderboard"].([]interface{})
	if len(lb) != 2 {
		t.Fatalf("leaderboard size = %d, want 2", len(lb))
	}
	first := lb[0].(map[string]interface{})
	if first["user_id"] == nil || first["win_rate"] != float64(100) {
		t.Fatalf("leaderboard[0] = %v, want the winner with 100%% win rate", first)
	}
}
