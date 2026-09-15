package ws

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestHubBroadcast 两个连接同房间都能收到广播；另一房间收不到。
func TestHubBroadcast(t *testing.T) {
	h := NewHub()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.Serve(w, r, 1)
	}))
	defer srv.Close()
	url := "ws://" + srv.Listener.Addr().String()

	dial := func() *websocket.Conn {
		t.Helper()
		c, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		return c
	}
	c1, c2 := dial(), dial()

	// 等两个连接都注册进房间（Serve 与 dial 并发，注册略晚于握手完成）
	deadline := time.Now().Add(2 * time.Second)
	for h.RoomSize(1) < 2 {
		if time.Now().After(deadline) {
			t.Fatalf("room size = %d, want 2", h.RoomSize(1))
		}
		time.Sleep(10 * time.Millisecond)
	}

	h.BroadcastToRoom(1, map[string]string{"type": "prop", "data": "x"})
	h.BroadcastToRoom(2, map[string]string{"type": "prop", "data": "other-room"}) // 不应收到

	read := func(c *websocket.Conn) map[string]string {
		_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, data, err := c.ReadMessage()
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		var m map[string]string
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		return m
	}
	for i, c := range []*websocket.Conn{c1, c2} {
		if got := read(c); got["type"] != "prop" || got["data"] != "x" {
			t.Fatalf("conn %d got %v", i, got)
		}
	}
	c1.Close()
	c2.Close()
}
