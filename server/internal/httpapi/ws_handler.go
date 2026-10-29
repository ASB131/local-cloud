package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/asb131/local-cloud/internal/ws"
)

func (s *Server) getWS(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer c.Close(websocket.StatusNormalClosure, "closed")

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	authCtx, authCancel := context.WithTimeout(ctx, 5*time.Second)
	var msg struct {
		Type        string `json:"type"`
		AccessToken string `json:"access_token"`
	}
	_, data, err := c.Read(authCtx)
	authCancel()
	if err != nil || json.Unmarshal(data, &msg) != nil || msg.Type != "auth" || msg.AccessToken == "" {
		_ = c.Close(websocket.StatusPolicyViolation, "auth required")
		return
	}
	claims, err := s.tokens.ParseAccess(msg.AccessToken)
	if err != nil {
		_ = c.Close(websocket.StatusPolicyViolation, "auth failed")
		return
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		_ = c.Close(websocket.StatusPolicyViolation, "auth failed")
		return
	}

	conn := &ws.Conn{UserID: userID, WS: c, Send: make(chan []byte, 32)}
	s.hub.Register(userID, conn)
	defer s.hub.Unregister(conn)

	go conn.WritePump(ctx)
	_ = c.Write(ctx, websocket.MessageText, []byte(`{"type":"ready"}`))

	for {
		if _, _, err := c.Read(ctx); err != nil {
			return
		}
	}
}
