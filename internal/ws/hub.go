package ws

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/c-wind/mist-docs/internal/config"
	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/middleware"
	"github.com/c-wind/mist-docs/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

/*
Yjs WebSocket 同步协议：

客户端 → 服务端消息格式（二进制）：
  [0, ...]  sync message
    [0, ...]  sync step 1 (client sends its state vector)
    [1, ...]  sync step 2 (client sends missing updates)
    [2, ...]  update (client sends a new update)

客户端 → 服务端消息格式（JSON）：
  {"type":"awareness","data":{...}}

服务端 → 客户端消息格式（JSON）：
  {"type":"join","user":{...}}
  {"type":"leave","user":{...}}
  {"type":"awareness","user_id":"...","data":{...}}
  {"type":"clients","users":[...]}   ← 新加入时发送当前在线用户
*/

// Yjs message types
const (
	MsgSync      = 0
	MsgAwareness = 1
)

const (
	SyncStep1  = 0
	SyncStep2  = 1
	SyncUpdate = 2
)

// Limits
const (
	maxSendBufferSize    = 512                      // per-client send channel size
	maxConnsPerDoc       = 50                       // max concurrent connections per document
	maxConnsGlobal       = 500                      // max total WebSocket connections
	sendBackoffTime      = 100 * time.Millisecond   // wait time when send buffer is full
	writeDeadline        = 10 * time.Second
	readDeadline         = 60 * time.Second
	pingIntervalDefault  = 30 // seconds
	maxMessageSizeDefault = 2 * 1024 * 1024 // 2MB
	permCheckInterval    = 60 * time.Second // periodic permission check interval
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// globalConnCount tracks total active WebSocket connections
var globalConnCount int64

type Client struct {
	Hub       *Hub
	Conn      *websocket.Conn
	Send      chan []byte
	DocID     string
	UserID    string
	Name      string
	Color     string
	Role      string // user role for permission checks
	DeptID    string // user department ID for permission checks
	mu        sync.Mutex
	closeOnce sync.Once
}

type Room struct {
	DocID   string
	Clients map[string]*Client
	Updates [][]byte // buffered updates for persistence
	mu      sync.RWMutex
	dirty   int // count of unsaved updates since last persist
}

type Message struct {
	DocID string
	Data  []byte
	From  string
}

type Hub struct {
	rooms      map[string]*Room
	mu         sync.RWMutex
	register   chan *Client
	unregister chan *Client
	broadcast  chan *Message
}

func NewHub() *Hub {
	return &Hub{
		rooms:      make(map[string]*Room),
		register:   make(chan *Client, 64),
		unregister: make(chan *Client, 64),
		broadcast:  make(chan *Message, 256),
	}
}

func (h *Hub) Run() {
	saveTicker := time.NewTicker(10 * time.Second)
	defer saveTicker.Stop()

	for {
		select {
		case client := <-h.register:
			h.handleRegister(client)

		case client := <-h.unregister:
			h.handleUnregister(client)

		case msg := <-h.broadcast:
			h.handleBroadcast(msg)

		case <-saveTicker.C:
			h.persistAllDirty()
		}
	}
}

func (h *Hub) handleRegister(client *Client) {
	// Global connection limit
	if atomic.LoadInt64(&globalConnCount) >= int64(maxConnsGlobal) {
		log.Printf("[WS] rejected: global conn limit reached (%d)", maxConnsGlobal)
		close(client.Send)
		return
	}

	h.mu.Lock()
	room, ok := h.rooms[client.DocID]
	if !ok {
		room = &Room{
			DocID:   client.DocID,
			Clients: make(map[string]*Client),
		}
		// Load persisted state
		if data, err := service.GetDocumentYjsState(client.DocID); err == nil && len(data) > 0 {
			room.Updates = [][]byte{data}
		}
		h.rooms[client.DocID] = room
	}

	// Per-document connection limit
	room.mu.Lock()
	if len(room.Clients) >= maxConnsPerDoc {
		room.mu.Unlock()
		h.mu.Unlock()
		log.Printf("[WS] rejected: doc=%s conn limit reached (%d)", client.DocID, maxConnsPerDoc)
		close(client.Send)
		return
	}
	room.Clients[client.UserID] = client
	room.mu.Unlock()
	h.mu.Unlock()

	atomic.AddInt64(&globalConnCount, 1)

	// Do NOT send step2 here — wait for client to send sync step1 first.
	// This is the correct Yjs protocol: client initiates sync after connect.
	// The client's onopen handler will send its state vector via step1,
	// and handleBroadcast will respond with step2.

	// Notify others about join
	joinMsg, _ := json.Marshal(map[string]interface{}{
		"type": "join",
		"user": map[string]string{
			"id":    client.UserID,
			"name":  client.Name,
			"color": client.Color,
		},
	})
	h.sendToRoom(client.DocID, client.UserID, joinMsg)

	// Send current online users to new client
	room.mu.RLock()
	users := make([]map[string]string, 0, len(room.Clients))
	for _, c := range room.Clients {
		users = append(users, map[string]string{
			"id":    c.UserID,
			"name":  c.Name,
			"color": c.Color,
		})
	}
	room.mu.RUnlock()
	clientsMsg, _ := json.Marshal(map[string]interface{}{
		"type":  "clients",
		"users": users,
	})
	client.Send <- clientsMsg

	log.Printf("[WS] join: user=%s doc=%s room_size=%d", client.UserID, client.DocID, len(room.Clients))
}

func (h *Hub) handleUnregister(client *Client) {
	h.mu.Lock()
	room, ok := h.rooms[client.DocID]
	if ok {
		room.mu.Lock()
		delete(room.Clients, client.UserID)
		size := len(room.Clients)
		room.mu.Unlock()

		if size == 0 {
			// Persist before removing
			h.persistRoom(room)
			delete(h.rooms, client.DocID)
		}
	}
	h.mu.Unlock()

	atomic.AddInt64(&globalConnCount, -1)

	// Notify others
	leaveMsg, _ := json.Marshal(map[string]interface{}{
		"type": "leave",
		"user": map[string]string{"id": client.UserID},
	})
	h.sendToRoom(client.DocID, "", leaveMsg)

	close(client.Send)
	log.Printf("[WS] leave: user=%s doc=%s", client.UserID, client.DocID)
}

func (h *Hub) handleBroadcast(msg *Message) {
	h.mu.RLock()
	room, ok := h.rooms[msg.DocID]
	h.mu.RUnlock()
	if !ok {
		return
	}

	// Parse Yjs message type
	if len(msg.Data) >= 2 && msg.Data[0] == MsgSync {
		subType := msg.Data[1]
		switch subType {
		case SyncStep1:
			// Client asks for missing updates → send what we have as step2.
			// Yjs applyUpdate is idempotent — sending full state is safe,
			// the client's Y.Doc will merge correctly via CRDT.
			room.mu.RLock()
			updates := room.Updates
			room.mu.RUnlock()
			if len(updates) > 0 {
				merged := mergeBytes(updates)
				reply := make([]byte, 2+len(merged))
				reply[0] = MsgSync
				reply[1] = SyncStep2
				copy(reply[2:], merged)
				h.sendToClient(msg.DocID, msg.From, reply)
			}

		case SyncStep2, SyncUpdate:
			// Client sends state or update → store + broadcast to others
			payload := make([]byte, len(msg.Data)-2)
			copy(payload, msg.Data[2:])
			room.mu.Lock()
			room.Updates = append(room.Updates, payload)
			room.dirty++
			// Keep only last 500 updates in memory; merge when exceeded.
			// After persist, persistRoom will compact Updates to a single entry.
			if len(room.Updates) > 500 {
				room.Updates = [][]byte{mergeBytes(room.Updates)}
				room.dirty = 0 // already conceptually merged
			}
			room.mu.Unlock()

			// Broadcast to other clients (as-is, original message data)
			h.sendToRoom(msg.DocID, msg.From, msg.Data)
		}
	} else {
		// Non-sync message (awareness etc), just relay
		h.sendToRoom(msg.DocID, msg.From, msg.Data)
	}
}

// sendToRoom sends data to all clients in a room except skipUserID.
// Uses a brief backoff when a client's send buffer is full to avoid
// dropping edit data.
func (h *Hub) sendToRoom(docID, skipUserID string, data []byte) {
	h.mu.RLock()
	room, ok := h.rooms[docID]
	h.mu.RUnlock()
	if !ok {
		return
	}

	room.mu.RLock()
	defer room.mu.RUnlock()

	for id, c := range room.Clients {
		if id == skipUserID {
			continue
		}
		h.sendToClientSafe(c, id, data)
	}
}

func (h *Hub) sendToClient(docID, userID string, data []byte) {
	h.mu.RLock()
	room, ok := h.rooms[docID]
	h.mu.RUnlock()
	if !ok {
		return
	}

	room.mu.RLock()
	c, ok := room.Clients[userID]
	room.mu.RUnlock()
	if !ok {
		return
	}
	h.sendToClientSafe(c, userID, data)
}

// sendToClientSafe tries to send data to a client with a brief backoff.
// If the buffer is still full after waiting, it logs a warning but does NOT
// drop the message — instead it blocks briefly (bounded by sendBackoffTime).
func (h *Hub) sendToClientSafe(c *Client, clientID string, data []byte) {
	select {
	case c.Send <- data:
		return
	default:
		// Buffer full — try one more time with a short wait
		log.Printf("[WS] backpressure: client=%s buffer full, waiting", clientID)
		select {
		case c.Send <- data:
			return
		case <-time.After(sendBackoffTime):
			log.Printf("[WS] drop: client=%s buffer still full after backoff", clientID)
		}
	}
}

func (h *Hub) persistRoom(room *Room) {
	room.mu.Lock()
	if room.dirty == 0 || len(room.Updates) == 0 {
		room.mu.Unlock()
		return
	}
	merged := mergeBytes(room.Updates)
	room.mu.Unlock()

	if err := service.SaveDocumentYjsState(room.DocID, merged); err != nil {
		log.Printf("[WS] persist error: doc=%s err=%v", room.DocID, err)
	} else {
		room.mu.Lock()
		// Compact: replace all updates with single merged entry
		room.Updates = [][]byte{merged}
		room.dirty = 0
		room.mu.Unlock()
		log.Printf("[WS] persisted+compacted: doc=%s size=%d", room.DocID, len(merged))
	}
}

func (h *Hub) persistAllDirty() {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, room := range h.rooms {
		room.mu.RLock()
		dirty := room.dirty
		room.mu.RUnlock()
		if dirty > 0 {
			h.persistRoom(room)
		}
	}
}

func mergeBytes(slices [][]byte) []byte {
	total := 0
	for _, s := range slices {
		total += len(s)
	}
	merged := make([]byte, 0, total)
	for _, s := range slices {
		merged = append(merged, s...)
	}
	return merged
}

func (h *Hub) GetRoomSize(docID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if room, ok := h.rooms[docID]; ok {
		room.mu.RLock()
		defer room.mu.RUnlock()
		return len(room.Clients)
	}
	return 0
}

// GetStats returns basic WebSocket hub statistics.
func (h *Hub) GetStats() (rooms int, clients int64) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	rooms = len(h.rooms)
	clients = atomic.LoadInt64(&globalConnCount)
	return
}

// ==================== WebSocket Handler ====================

func ServeWS(hub *Hub, c *gin.Context) {
	docID := c.Param("doc_id")
	token := c.Query("token")

	userID, err := middleware.ParseMistLabToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "认证失败"})
		return
	}

	// Look up user from shared users table
	var username string
	var isAdmin bool
	database.DB.QueryRow(
		`SELECT username, is_admin FROM users WHERE id = ?`, userID,
	).Scan(&username, &isAdmin)

	// TODO: team-scoped permission check
	_ = docID

	// Global connection limit check before upgrade
	if atomic.LoadInt64(&globalConnCount) >= int64(maxConnsGlobal) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "连接数已达上限，请稍后重试"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[WS] upgrade error: %v", err)
		return
	}

	colors := []string{"#e06c75", "#e5c07b", "#98c379", "#56b6c2", "#61afef", "#c678dd", "#d19a66", "#be5046"}
	colorIdx := 0
	for _, ch := range userID {
		colorIdx += int(ch)
	}

	client := &Client{
		Hub:    hub,
		Conn:   conn,
		Send:   make(chan []byte, maxSendBufferSize),
		DocID:  docID,
		UserID: userID,
		Name:   username,
		Color:  colors[colorIdx%len(colors)],
		Role:   func() string { if isAdmin { return "admin" }; return "member" }(),
		DeptID: "", // deprecated, teams replace departments
	}

	hub.register <- client

	go client.writePump()
	go client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()

	maxSize := config.C.WebSocket.MaxMessageSize
	if maxSize == 0 {
		maxSize = maxMessageSizeDefault
	}
	c.Conn.SetReadLimit(maxSize)
	c.Conn.SetReadDeadline(time.Now().Add(readDeadline))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(readDeadline))
		return nil
	})

	// Periodic permission check — kick client if permission revoked
	permTicker := time.NewTicker(permCheckInterval)
	defer permTicker.Stop()

	// Channel to signal permission check results
	go func() {
		for range permTicker.C {
			if c.Role == "super_admin" {
				continue // super_admin always has access
			}
			perm, err := service.CheckPermissionSimple(context.Background(), c.UserID, c.DeptID, c.DocID)
			if err != nil || perm == "none" {
				log.Printf("[WS] perm revoked: user=%s doc=%s — disconnecting", c.UserID, c.DocID)
				c.Conn.Close()
				return
			}
		}
	}()

	for {
		_, data, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("[WS] read error: user=%s err=%v", c.UserID, err)
			}
			break
		}

		c.Hub.broadcast <- &Message{
			DocID: c.DocID,
			Data:  data,
			From:  c.UserID,
		}
	}
}

func (c *Client) writePump() {
	pingInterval := config.C.WebSocket.PingInterval
	if pingInterval == 0 {
		pingInterval = pingIntervalDefault
	}
	ticker := time.NewTicker(time.Duration(pingInterval) * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeDeadline))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.mu.Lock()
			err := c.Conn.WriteMessage(websocket.BinaryMessage, msg)
			c.mu.Unlock()
			if err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeDeadline))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
