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

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// Client represents a connected editor
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

// Room represents a document editing session
type Room struct {
	DocID   string
	Clients map[string]*Client
	State   []byte // persisted Yjs state
	mu      sync.RWMutex
}

// Message is a broadcast message
type Message struct {
	DocID string
	Data  []byte
	From  string // client ID to skip
}

// Hub manages all rooms
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
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan *Message, 256),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			room, ok := h.rooms[client.DocID]
			if !ok {
				room = &Room{
					DocID:   client.DocID,
					Clients: make(map[string]*Client),
				}
				// Load persisted state from file
				if data, err := service.GetDocumentYjsState(client.DocID); err == nil && data != nil {
					room.State = data
				}
				h.rooms[client.DocID] = room
			}
			room.mu.Lock()
			room.Clients[client.UserID] = client
			room.mu.Unlock()
			h.mu.Unlock()

			// Send current state to new client
			if len(room.State) > 0 {
				client.Send <- room.State
			}

			// Notify others about join
			h.broadcastPresence(client, "join")
			log.Printf("[WS] user %s (%s) joined doc %s (room: %d)", client.UserID, client.Name, client.DocID, len(room.Clients))

		case client := <-h.unregister:
			h.mu.Lock()
			room, ok := h.rooms[client.DocID]
			if ok {
				room.mu.Lock()
				delete(room.Clients, client.UserID)
				size := len(room.Clients)
				room.mu.Unlock()

				if size == 0 {
					// Persist state before removing room
					if len(room.State) > 0 {
						service.SaveDocumentYjsState(client.DocID, room.State)
					}
					delete(h.rooms, client.DocID)
				}
			}
			h.mu.Unlock()

			h.broadcastPresence(client, "leave")
			close(client.Send)
			log.Printf("[WS] user %s left doc %s", client.UserID, client.DocID)

		case msg := <-h.broadcast:
			h.mu.RLock()
			room, ok := h.rooms[msg.DocID]
			h.mu.RUnlock()
			if ok {
				// Update room state
				room.mu.Lock()
				room.State = append(room.State, msg.Data...)
				room.mu.Unlock()

				// Broadcast to others
				room.mu.RLock()
				for id, c := range room.Clients {
					if id != msg.From {
						select {
						case c.Send <- msg.Data:
						default:
							log.Printf("[WS] client %s send buffer full, dropping", id)
						}
					}
				}
				room.mu.RUnlock()

				// Periodically persist (every ~30 messages)
				if len(room.State)%30 == 0 {
					go service.SaveDocumentYjsState(msg.DocID, room.State)
				}
			}
		}
	}
}

func (h *Hub) broadcastPresence(client *Client, eventType string) {
	msg, _ := json.Marshal(map[string]interface{}{
		"type": eventType,
		"user": map[string]string{
			"id":    client.UserID,
			"name":  client.Name,
			"color": client.Color,
		},
	})

	h.mu.RLock()
	room, ok := h.rooms[client.DocID]
	h.mu.RUnlock()
	if ok {
		room.mu.RLock()
		for id, c := range room.Clients {
			if id != client.UserID {
				select {
				case c.Send <- msg:
				default:
				}
			}
		}
		room.mu.RUnlock()
	}
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

func ServeWS(hub *Hub, c *gin.Context) {
	docID := c.Param("doc_id")
	token := c.Query("token")

	// Validate JWT
	claims, err := middleware.ParseToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "认证失败"})
		return
	}

	// Check permission
	perm := "none"
	if claims.Role == "super_admin" {
		perm = "write"
	} else {
		perm, _ = service.CheckPermissionSimple(c.Request.Context(), claims.UserID, claims.DepartmentID, docID)
	}
	if perm == "none" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[WS] upgrade error: %v", err)
		return
	}

	// Assign color for cursor
	colors := []string{"#f44336", "#e91e63", "#9c27b0", "#2196f3", "#00bcd4", "#4caf50", "#ff9800", "#ff5722"}
	colorIdx := 0
	for _, c := range claims.UserID {
		colorIdx += int(c)
	}
	color := colors[colorIdx%len(colors)]

	client := &Client{
		Hub:    hub,
		Conn:   conn,
		Send:   make(chan []byte, 256),
		DocID:  docID,
		UserID: claims.UserID,
		Name:   claims.Username,
		Color:  color,
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

	c.Conn.SetReadLimit(config.C.WebSocket.MaxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("[WS] read error: %v", err)
			}
			break
		}
		c.Hub.broadcast <- &Message{
			DocID: c.DocID,
			Data:  message,
			From:  c.UserID,
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(time.Duration(config.C.WebSocket.PingInterval) * time.Second)
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
			err := c.Conn.WriteMessage(websocket.TextMessage, msg)
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