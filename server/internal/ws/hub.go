package ws

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

type Hub struct {
	mu    sync.RWMutex
	conns map[uuid.UUID]map[*Conn]struct{}
}

type Conn struct {
	UserID uuid.UUID
	WS     *websocket.Conn
	Send   chan []byte
}

func NewHub() *Hub {
	return &Hub{conns: make(map[uuid.UUID]map[*Conn]struct{})}
}

func (h *Hub) Register(userID uuid.UUID, c *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.conns[userID] == nil {
		h.conns[userID] = make(map[*Conn]struct{})
	}
	h.conns[userID][c] = struct{}{}
}

func (h *Hub) Unregister(c *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	set := h.conns[c.UserID]
	delete(set, c)
	if len(set) == 0 {
		delete(h.conns, c.UserID)
	}
	close(c.Send)
}

func (h *Hub) Broadcast(userID uuid.UUID, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.conns[userID] {
		select {
		case c.Send <- data:
		default:
		}
	}
}

func (c *Conn) WritePump(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case msg, ok := <-c.Send:
			if !ok {
				return
			}
			wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := c.WS.Write(wctx, websocket.MessageText, msg)
			cancel()
			if err != nil {
				return
			}
		case <-ticker.C:
			wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := c.WS.Write(wctx, websocket.MessageText, []byte(`{"type":"ping"}`))
			cancel()
			if err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
