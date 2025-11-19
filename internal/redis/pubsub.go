package redis

import (
	"go-websocket/internal/models"
	"go-websocket/internal/ws"
	"log/slog"

	"github.com/goccy/go-json"
)

func SubscribeToEvents(client *Client, hub *ws.Hub) {
	slog.Info("[REDIS] Starting Redis pub/sub subscription...")

	// Subscribe to all channel events using pattern
	pubsub := client.rdb.PSubscribe(client.ctx, "channel:*")
	defer pubsub.Close()

	slog.Info("[REDIS] Subscribed to Redis pub/sub", "pattern", "channel:*")

	// Wait for subscription confirmation
	_, err := pubsub.Receive(client.ctx)
	if err != nil {
		slog.Error("[REDIS] Failed to receive subscription confirmation", "error", err)
		return // Or panic/fatal depending on requirements
	}

	slog.Info("[REDIS] Subscription confirmed, listening for messages...")

	// Listen for messages
	ch := pubsub.Channel()

	for msg := range ch {
		// slog.Debug("[REDIS] Received message from Redis", "channel", msg.Channel, "size", len(msg.Payload))

		var event models.Event
		if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
			slog.Error("[REDIS] Error unmarshaling event", "channel", msg.Channel, "error", err, "payload", msg.Payload)
			continue
		}

		// slog.Debug("[REDIS] Event parsed successfully", "type", event.Type, "channelId", event.ChannelId, "timestamp", event.Timestamp)

		// Convert to broadcast message
		broadcastMsg := &models.BroadcastMessage{
			ChannelId: event.ChannelId,
			Payload:   []byte(msg.Payload),
		}

		// slog.Debug("[REDIS] Sending broadcast message to hub", "channelId", event.ChannelId)

		// Send to hub for broadcasting to WebSocket clients
		hub.Broadcast <- broadcastMsg

		// slog.Debug("[REDIS] Broadcast message sent to hub successfully")
	}

	slog.Info("[REDIS] Redis pub/sub channel closed")
}
