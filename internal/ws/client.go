package ws

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/goccy/go-json"
	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message
	writeWait = 10 * time.Second

	// Time allowed to read next pong message
	pongWait = 60 * time.Second

	// Send pings with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Max message size
	maxMessageSize = 512 * 1024 // 512 KB
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// TODO: Validate origin in production
		return true
	},
}

type Client struct {
	hub       *Hub
	conn      *websocket.Conn
	send      chan []byte
	channelId string
	userId    string
	userName  string
}

// ReadPump pumps messages from WebSocket to hub
func (c *Client) ReadPump() {
	slog.Debug("[CLIENT] ReadPump started", "user", c.userId, "channel", c.channelId)
	defer func() {
		slog.Debug("[CLIENT] ReadPump stopped", "user", c.userId, "channel", c.channelId)
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		// slog.Debug("[CLIENT] Received pong", "user", c.userId, "channel", c.channelId)
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Warn("[CLIENT] Unexpected close error", "user", c.userId, "channel", c.channelId, "error", err)
			} else {
				slog.Info("[CLIENT] Connection closed", "user", c.userId, "channel", c.channelId, "error", err)
			}
			break
		}

		// slog.Debug("[CLIENT] Received message", "user", c.userId, "channel", c.channelId, "size", len(message))

		// Handle client-sent events (typing, etc.)
		c.handleClientMessage(message)
	}
}

// WritePump pumps messages from hub to WebSocket
func (c *Client) WritePump() {
	slog.Debug("[CLIENT] WritePump started", "user", c.userId, "channel", c.channelId)
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		slog.Debug("[CLIENT] WritePump stopped", "user", c.userId, "channel", c.channelId)
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				slog.Info("[CLIENT] Send channel closed, closing connection", "user", c.userId, "channel", c.channelId)
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				slog.Error("[CLIENT] Error getting writer", "user", c.userId, "channel", c.channelId, "error", err)
				return
			}
			w.Write(message)

			// slog.Debug("[CLIENT] Sent message", "user", c.userId, "channel", c.channelId, "size", len(message))

			if err := w.Close(); err != nil {
				slog.Error("[CLIENT] Error closing writer", "user", c.userId, "channel", c.channelId, "error", err)
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				slog.Error("[CLIENT] Error sending ping", "user", c.userId, "channel", c.channelId, "error", err)
				return
			}
			// slog.Debug("[CLIENT] Sent ping", "user", c.userId, "channel", c.channelId)
		}
	}
}

func (c *Client) handleClientMessage(message []byte) {
	// slog.Debug("[CLIENT] Handling client message", "user", c.userId, "channel", c.channelId, "payload", string(message))

	var msg map[string]interface{}
	if err := json.Unmarshal(message, &msg); err != nil {
		slog.Error("[CLIENT] Error unmarshaling message", "user", c.userId, "channel", c.channelId, "error", err)
		return
	}

	eventType, ok := msg["type"].(string)
	if !ok {
		slog.Warn("[CLIENT] No 'type' field in message", "user", c.userId, "channel", c.channelId)
		return
	}

	// slog.Debug("[CLIENT] Processing event type", "type", eventType, "user", c.userId, "channel", c.channelId)

	switch eventType {
	case "typing:start":
		if err := c.hub.redisClient.PublishTypingStart(c.channelId, c.userId, c.userName); err != nil {
			slog.Error("[CLIENT] Error publishing typing:start", "error", err)
		} else {
			// slog.Debug("[CLIENT] Published typing:start", "user", c.userId, "channel", c.channelId)
		}

	case "typing:stop":
		if err := c.hub.redisClient.PublishTypingStop(c.channelId, c.userId); err != nil {
			slog.Error("[CLIENT] Error publishing typing:stop", "error", err)
		} else {
			// slog.Debug("[CLIENT] Published typing:stop", "user", c.userId, "channel", c.channelId)
		}

	default:
		slog.Warn("[CLIENT] Unknown event type", "type", eventType, "user", c.userId, "channel", c.channelId)
	}
}
