package ws

import (
	"log"
	"sync"

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
}

// Room represents a document editing session
type Room struct {
	DocID    string
	Clients  map[string]*Client
	State    []byte  // persisted Yjs state
	mu       sync.RWMutex
}

// Hub manages all rooms
type Hub struct {
	rooms   map[string]*Room
	mu      sync.RWMutex
	register   chan *Client
	unregister chan *Client
	broadcast  chan *Message
}

type Message struct {
	DocID  string
	Data   []byte
	From   string // client ID to skip
}

func NewHub() *Hub {
	return &Hub{
		rooms:      make(map[string]*Room),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan *Message),
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
				// TODO: load persisted state from database
				h.rooms[client.DocID] = room
			}
			room.mu.Lock()
			room.Clients[client.UserID] = client
			room.mu.Unlock()
			h.mu.Unlock()

			// Notify others
			h.broadcastJoin(client)
			log.Printf("[WS] user %s joined doc %s (room size: %d)", client.UserID, client.DocID, len(room.Clients))

		case client := <-h.unregister:
			h.mu.Lock()
			room, ok := h.rooms[client.DocID]
			if ok {
				room.mu.Lock()
				delete(room.Clients, client.UserID)
				size := len(room.Clients)
				room.mu.Unlock()
				if size == 0 {
					// TODO: persist state before removing room
					delete(h.rooms, client.DocID)
				}
			}
			h.mu.Unlock()

			// Notify others
			h.broadcastLeave(client)
			close(client.Send)
			log.Printf("[WS] user %s left doc %s", client.UserID, client.DocID)

		case msg := <-h.broadcast:
			h.mu.RLock()
			room, ok := h.rooms[msg.DocID]
			h.mu.RUnlock()
			if ok {
				room.mu.RLock()
				for id, c := range room.Clients {
					if id != msg.From {
						select {
						case c.Send <- msg.Data:
						default:
							// client stuck, will be cleaned up
						}
					}
				}
				room.mu.RUnlock()
			}
		}
	}
}

func (h *Hub) broadcastJoin(client *Client) {
	// TODO: send join notification with user info
}

func (h *Hub) broadcastLeave(client *Client) {
	// TODO: send leave notification
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
	// TODO: validate JWT token from query
	docID := c.Param("doc_id")
	token := c.Query("token")
	_ = token // validate later

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[WS] upgrade error: %v", err)
		return
	}

	client := &Client{
		Hub:    hub,
		Conn:   conn,
		Send:   make(chan []byte, 256),
		DocID:  docID,
		UserID: "temp", // TODO: from JWT
		Name:   "临时用户",
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
	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
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
	defer c.Conn.Close()
	for msg := range c.Send {
		if err := c.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			break
		}
	}
}
