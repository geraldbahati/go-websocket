package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

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
	log.Printf("[CLIENT] ReadPump started (user: %s, channel: %s)", c.userId, c.channelId)
	defer func() {
		log.Printf("[CLIENT] ReadPump stopped (user: %s, channel: %s)", c.userId, c.channelId)
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		log.Printf("[CLIENT] Received pong from client (user: %s, channel: %s)", c.userId, c.channelId)
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[CLIENT] Unexpected close error (user: %s, channel: %s): %v", c.userId, c.channelId, err)
			} else {
				log.Printf("[CLIENT] Connection closed (user: %s, channel: %s): %v", c.userId, c.channelId, err)
			}
			break
		}

		log.Printf("[CLIENT] Received message from client (user: %s, channel: %s, size: %d bytes)", c.userId, c.channelId, len(message))

		// Handle client-sent events (typing, etc.)
		c.handleClientMessage(message)
	}
}

// WritePump pumps messages from hub to WebSocket
func (c *Client) WritePump() {
	log.Printf("[CLIENT] WritePump started (user: %s, channel: %s)", c.userId, c.channelId)
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		log.Printf("[CLIENT] WritePump stopped (user: %s, channel: %s)", c.userId, c.channelId)
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				log.Printf("[CLIENT] Send channel closed, closing connection (user: %s, channel: %s)", c.userId, c.channelId)
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				log.Printf("[CLIENT] Error getting writer (user: %s, channel: %s): %v", c.userId, c.channelId, err)
				return
			}
			w.Write(message)

			log.Printf("[CLIENT] Sent message to client (user: %s, channel: %s, size: %d bytes)", c.userId, c.channelId, len(message))

			if err := w.Close(); err != nil {
				log.Printf("[CLIENT] Error closing writer (user: %s, channel: %s): %v", c.userId, c.channelId, err)
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("[CLIENT] Error sending ping (user: %s, channel: %s): %v", c.userId, c.channelId, err)
				return
			}
			log.Printf("[CLIENT] Sent ping to client (user: %s, channel: %s)", c.userId, c.channelId)
		}
	}
}

func (c *Client) handleClientMessage(message []byte) {
	log.Printf("[CLIENT] Handling client message (user: %s, channel: %s): %s", c.userId, c.channelId, string(message))

	var msg map[string]interface{}
	if err := json.Unmarshal(message, &msg); err != nil {
		log.Printf("[CLIENT] Error unmarshaling message (user: %s, channel: %s): %v", c.userId, c.channelId, err)
		return
	}

	eventType, ok := msg["type"].(string)
	if !ok {
		log.Printf("[CLIENT] No 'type' field in message (user: %s, channel: %s)", c.userId, c.channelId)
		return
	}

	log.Printf("[CLIENT] Processing event type: %s (user: %s, channel: %s)", eventType, c.userId, c.channelId)

	switch eventType {
	case "typing:start":
		if err := c.hub.redisClient.PublishTypingStart(c.channelId, c.userId, c.userName); err != nil {
			log.Printf("[CLIENT] Error publishing typing:start: %v", err)
		} else {
			log.Printf("[CLIENT] Published typing:start (user: %s, channel: %s)", c.userId, c.channelId)
		}

	case "typing:stop":
		if err := c.hub.redisClient.PublishTypingStop(c.channelId, c.userId); err != nil {
			log.Printf("[CLIENT] Error publishing typing:stop: %v", err)
		} else {
			log.Printf("[CLIENT] Published typing:stop (user: %s, channel: %s)", c.userId, c.channelId)
		}

	default:
		log.Printf("[CLIENT] Unknown event type: %s (user: %s, channel: %s)", eventType, c.userId, c.channelId)
	}
}
