package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/c-wind/mist-docs/internal/config"
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
	SyncStep1 = 0
	SyncStep2 = 1
	SyncUpdate = 2
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type Client struct {
	Hub    *Hub
	Conn   *websocket.Conn
	Send   chan []byte
	DocID  string
	UserID string
	Name   string
	Color  string
	mu     sync.Mutex
}

type Room struct {
	DocID       string
	Clients     map[string]*Client
	StateVector []byte   // latest known state vector
	Updates     [][]byte // buffered updates for persistence
	mu          sync.RWMutex
	dirty       int // count of unsaved updates
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
	room.mu.Lock()
	room.Clients[client.UserID] = client
	room.mu.Unlock()
	h.mu.Unlock()

	// Send existing state to new client
	if len(room.Updates) > 0 {
		// Merge all updates and send as sync step 2
		merged := mergeBytes(room.Updates)
		msg := append([]byte{MsgSync, SyncStep2}, merged...)
		client.Send <- msg
	}

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
	users := []map[string]string{}
	for _, c := range room.Clients {
		users = append(users, map[string]string{
			"id":    c.UserID,
			"name":  c.Name,
			"color": c.Color,
		})
	}
	room.mu.RUnlock()
	clientsMsg, _ := json.Marshal(map[string]interface{}{
		"type":   "clients",
		"users":  users,
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
			// Client asks for missing updates → send what we have
			if len(room.Updates) > 0 {
				merged := mergeBytes(room.Updates)
				reply := append([]byte{MsgSync, SyncStep2}, merged...)
				// Send only to the originating client
				h.sendToClient(msg.DocID, msg.From, reply)
			}

		case SyncStep2, SyncUpdate:
			// Client sends state or update → store + broadcast to others
			payload := msg.Data[2:]
			room.mu.Lock()
			room.Updates = append(room.Updates, payload)
			room.dirty++
			// Keep only last 1000 updates in memory, merge periodically
			if len(room.Updates) > 1000 {
				room.Updates = [][]byte{mergeBytes(room.Updates)}
			}
			room.mu.Unlock()

			// Broadcast to other clients
			h.sendToRoom(msg.DocID, msg.From, msg.Data)
		}
	} else {
		// Non-sync message (awareness etc), just relay
		h.sendToRoom(msg.DocID, msg.From, msg.Data)
	}
}

func (h *Hub) sendToRoom(docID, skipUserID string, data []byte) {
	h.mu.RLock()
	room, ok := h.rooms[docID]
	h.mu.RUnlock()
	if !ok {
		return
	}

	room.mu.RLock()
	for id, c := range room.Clients {
		if id != skipUserID {
			select {
			case c.Send <- data:
			default:
				log.Printf("[WS] drop: client=%s buffer full", id)
			}
		}
	}
	room.mu.RUnlock()
}

func (h *Hub) sendToClient(docID, userID string, data []byte) {
	h.mu.RLock()
	room, ok := h.rooms[docID]
	h.mu.RUnlock()
	if !ok {
		return
	}

	room.mu.RLock()
	if c, ok := room.Clients[userID]; ok {
		select {
		case c.Send <- data:
		default:
		}
	}
	room.mu.RUnlock()
}

func (h *Hub) persistRoom(room *Room) {
	room.mu.RLock()
	if room.dirty == 0 || len(room.Updates) == 0 {
		room.mu.RUnlock()
		return
	}
	merged := mergeBytes(room.Updates)
	room.mu.RUnlock()

	if err := service.SaveDocumentYjsState(room.DocID, merged); err != nil {
		log.Printf("[WS] persist error: doc=%s err=%v", room.DocID, err)
	} else {
		room.mu.Lock()
		room.dirty = 0
		room.mu.Unlock()
		log.Printf("[WS] persisted: doc=%s size=%d", room.DocID, len(merged))
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

// ==================== WebSocket Handler ====================

func ServeWS(hub *Hub, c *gin.Context) {
	docID := c.Param("doc_id")
	token := c.Query("token")

	claims, err := middleware.ParseToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "认证失败"})
		return
	}

	// Permission check
	if claims.Role != "super_admin" {
		perm, err := service.CheckPermissionSimple(c.Request.Context(), claims.UserID, claims.DepartmentID, docID)
		if err != nil || perm == "none" {
			c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
			return
		}
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[WS] upgrade error: %v", err)
		return
	}

	colors := []string{"#e06c75", "#e5c07b", "#98c379", "#56b6c2", "#61afef", "#c678dd", "#d19a66", "#be5046"}
	colorIdx := 0
	for _, ch := range claims.UserID {
		colorIdx += int(ch)
	}

	client := &Client{
		Hub:    hub,
		Conn:   conn,
		Send:   make(chan []byte, 256),
		DocID:  docID,
		UserID: claims.UserID,
		Name:   claims.Username,
		Color:  colors[colorIdx%len(colors)],
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
		maxSize = 1048576
	}
	c.Conn.SetReadLimit(maxSize)
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		msgType, data, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("[WS] read error: user=%s err=%v", c.UserID, err)
			}
			break
		}

		// Only handle binary (Yjs sync) and text (awareness JSON)
		_ = msgType

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
		pingInterval = 30
	}
	ticker := time.NewTicker(time.Duration(pingInterval) * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
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
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
