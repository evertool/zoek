// Package ws — 房间长连接（WebSocket）推送。
//
// 设计：动作仍走 REST（保留鉴权/幂等/审计），WS 只做服务端 → 客户端的即时推送：
//   - {"type":"prop","data":{...}}       道具事件（即时回放动画）
//   - {"type":"game"}                    牌局状态变化（客户端重拉 /games/:id）
//   - {"type":"ledger"}                  流水变化（客户端重拉 /games/:id/adjustments）
//   - {"type":"ping"}                    服务端心跳（25s 一次）
//
// 客户端发来的消息仅用于保活（任意内容都会刷新读超时）。
//
// 连接按 game_id 分房间；同一个玩家开两个页面会各占一条连接，广播都发。
package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second // 读超时：60s 内无任何消息（含保活）则断开
	pingPeriod = 25 * time.Second // 服务端心跳间隔（必须小于 pongWait）
	sendBuffer = 16               // 每连接发送缓冲；满了直接丢消息（推送可容忍）
)

// Conn is one live room connection (already authenticated & room-bound).
type Conn struct {
	ws     *websocket.Conn
	send   chan []byte
	roomID int64
}

// Hub keeps room → connections registry.
type Hub struct {
	mu    sync.RWMutex
	rooms map[int64]map[*Conn]struct{}
}

// DefaultHub is the process-wide hub used by handlers to broadcast.
var Default = NewHub()

// NewHub creates a hub. Call Run() once to start the cleanup loop.
func NewHub() *Hub {
	return &Hub{rooms: make(map[int64]map[*Conn]struct{})}
}

func (h *Hub) register(roomID int64, c *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[roomID] == nil {
		h.rooms[roomID] = make(map[*Conn]struct{})
	}
	h.rooms[roomID][c] = struct{}{}
}

func (h *Hub) unregister(roomID int64, c *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if set := h.rooms[roomID]; set != nil {
		if _, ok := set[c]; ok {
			delete(set, c)
			close(c.send)
		}
		if len(set) == 0 {
			delete(h.rooms, roomID)
		}
	}
}

// RoomSize returns the number of live connections in a room (for tests/logs).
func (h *Hub) RoomSize(roomID int64) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms[roomID])
}

// BroadcastToRoom marshals v and pushes it to every connection in the room.
// Non-blocking: a slow/full client drops the message instead of stalling others.
func (h *Hub) BroadcastToRoom(roomID int64, v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.rooms[roomID] {
		select {
		case c.send <- data:
		default: // 缓冲满：丢弃（客户端还有轮询兜底）
		}
	}
}

// Emit 推送一条带类型的房间事件：{"type": typ, "data": data}
func Emit(roomID int64, typ string, data interface{}) {
	Default.BroadcastToRoom(roomID, map[string]interface{}{"type": typ, "data": data})
}

// Dirty 推送一条无载荷的刷新信号（客户端按 type 重拉对应数据）
func Dirty(roomID int64, typ string) {
	Emit(roomID, typ, nil)
}

// Serve upgrades an authenticated, room-bound request and blocks until the
// connection dies. Call from the HTTP handler goroutine.
func (h *Hub) Serve(w http.ResponseWriter, r *http.Request, roomID int64) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		// 小程序客户端不发 Origin，放行（鉴权已由 token 保证）
		CheckOrigin: func(*http.Request) bool { return true },
	}
	raw, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("ws upgrade:", err)
		return
	}
	c := &Conn{ws: raw, send: make(chan []byte, sendBuffer), roomID: roomID}
	h.register(roomID, c)

	go c.writePump()
	c.readPump() // 阻塞直到连接断开
	h.unregister(roomID, c)
}

// readPump drains client messages (keepalive only) until the socket dies.
func (c *Conn) readPump() {
	defer c.ws.Close()
	c.ws.SetReadLimit(512)
	_ = c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error {
		_ = c.ws.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		if _, _, err := c.ws.ReadMessage(); err != nil {
			return
		}
		_ = c.ws.SetReadDeadline(time.Now().Add(pongWait))
	}
}

// writePump flushes outbound messages + server heartbeat.
func (c *Conn) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.ws.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.ws.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			if err := c.ws.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.ws.WriteMessage(websocket.TextMessage, []byte(`{"type":"ping"}`)); err != nil {
				return
			}
		}
	}
}
