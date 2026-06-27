package ws

import (
	"encoding/json"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

type Event struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type HubStats struct {
	ActiveClients          int    `json:"activeClients"`
	BroadcastQueueDepth    int    `json:"broadcastQueueDepth"`
	BroadcastQueueCapacity int    `json:"broadcastQueueCapacity"`
	DroppedBroadcasts      uint64 `json:"droppedBroadcasts"`
}

type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

type Hub struct {
	clients           map[*Client]bool
	register          chan *Client
	unregister        chan *Client
	broadcast         chan []byte
	activeClients     atomic.Int64
	droppedBroadcasts atomic.Uint64
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte, 256),
	}
}

func (h *Hub) Run() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case client := <-h.register:
			h.registerClient(client)
		case client := <-h.unregister:
			h.unregisterClient(client)
		case message := <-h.broadcast:
			h.broadcastMessage(message)
		case <-ticker.C:
			h.Broadcast(Event{Type: "heartbeat", Data: map[string]string{"time": time.Now().Format(time.RFC3339)}})
		}
	}
}

func (h *Hub) registerClient(client *Client) {
	if _, exists := h.clients[client]; exists {
		return
	}
	h.clients[client] = true
	h.activeClients.Add(1)
}

func (h *Hub) unregisterClient(client *Client) {
	if _, ok := h.clients[client]; !ok {
		return
	}
	delete(h.clients, client)
	close(client.send)
	h.activeClients.Add(-1)
}

func (h *Hub) broadcastMessage(message []byte) {
	for client := range h.clients {
		select {
		case client.send <- message:
		default:
			h.unregisterClient(client)
			h.droppedBroadcasts.Add(1)
		}
	}
}

func (h *Hub) Broadcast(event Event) bool {
	data, err := json.Marshal(event)
	if err != nil {
		h.droppedBroadcasts.Add(1)
		return false
	}
	select {
	case h.broadcast <- data:
		return true
	default:
		h.droppedBroadcasts.Add(1)
		return false
	}
}

func (h *Hub) Stats() HubStats {
	return HubStats{
		ActiveClients:          int(h.activeClients.Load()),
		BroadcastQueueDepth:    len(h.broadcast),
		BroadcastQueueCapacity: cap(h.broadcast),
		DroppedBroadcasts:      h.droppedBroadcasts.Load(),
	}
}

func (h *Hub) Register(conn *websocket.Conn) *Client {
	client := &Client{hub: h, conn: conn, send: make(chan []byte, 256)}
	h.register <- client
	return client
}

func (c *Client) ReadPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(4096)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	defer c.conn.Close()
	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
