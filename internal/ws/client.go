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
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Warn("[CLIENT] Unexpected close", "user", c.userId, "channel", c.channelId, "error", err)
			}
			break
		}

		c.handleClientMessage(message)
	}
}

// WritePump pumps messages from hub to WebSocket
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				slog.Error("[CLIENT] Failed to get writer", "user", c.userId, "channel", c.channelId, "error", err)
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				slog.Error("[CLIENT] Failed to close writer", "user", c.userId, "channel", c.channelId, "error", err)
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				slog.Error("[CLIENT] Failed to send ping", "user", c.userId, "channel", c.channelId, "error", err)
				return
			}
		}
	}
}

func (c *Client) handleClientMessage(message []byte) {
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

	switch eventType {
	case "typing:start":
		var threadId *string
		if data, ok := msg["data"].(map[string]interface{}); ok {
			if tid, ok := data["threadId"].(string); ok && tid != "" {
				threadId = &tid
			}
		}

		if err := c.hub.redisClient.PublishTypingStart(c.channelId, c.userId, c.userName, threadId); err != nil {
			slog.Error("[CLIENT] Failed to publish typing:start", "user", c.userId, "channel", c.channelId, "error", err)
		}

	case "typing:stop":
		var threadId *string
		if data, ok := msg["data"].(map[string]interface{}); ok {
			if tid, ok := data["threadId"].(string); ok && tid != "" {
				threadId = &tid
			}
		}

		if err := c.hub.redisClient.PublishTypingStop(c.channelId, c.userId, threadId); err != nil {
			slog.Error("[CLIENT] Failed to publish typing:stop", "user", c.userId, "channel", c.channelId, "error", err)
		}

	default:
		slog.Warn("[CLIENT] Unknown event type", "type", eventType, "user", c.userId, "channel", c.channelId)
	}
}
