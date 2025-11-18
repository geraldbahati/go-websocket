package redis

import (
	"encoding/json"
	"go-websocket/internal/models"
	"go-websocket/internal/ws"
	"log"
)

func SubscribeToEvents(client *Client, hub *ws.Hub) {
	log.Println("[REDIS] Starting Redis pub/sub subscription...")

	// Subscribe to all channel events using pattern
	pubsub := client.rdb.PSubscribe(client.ctx, "channel:*")
	defer pubsub.Close()

	log.Println("[REDIS] Subscribed to Redis pub/sub (pattern: channel:*)")

	// Wait for subscription confirmation
	_, err := pubsub.Receive(client.ctx)
	if err != nil {
		log.Fatalf("[REDIS] Failed to receive subscription confirmation: %v", err)
	}

	log.Println("[REDIS] Subscription confirmed, listening for messages...")

	// Listen for messages
	ch := pubsub.Channel()

	for msg := range ch {
		log.Printf("[REDIS] Received message from Redis (channel: %s, payload size: %d bytes)", msg.Channel, len(msg.Payload))

		var event models.Event
		if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
			log.Printf("[REDIS] Error unmarshaling event from channel %s: %v", msg.Channel, err)
			log.Printf("[REDIS] Payload was: %s", msg.Payload)
			continue
		}

		log.Printf("[REDIS] Event parsed successfully (type: %s, channelId: %s, timestamp: %d)", event.Type, event.ChannelId, event.Timestamp)

		// Convert to broadcast message
		broadcastMsg := &models.BroadcastMessage{
			ChannelId: event.ChannelId,
			Payload:   []byte(msg.Payload),
		}

		log.Printf("[REDIS] Sending broadcast message to hub (channelId: %s)", event.ChannelId)

		// Send to hub for broadcasting to WebSocket clients
		hub.Broadcast <- broadcastMsg

		log.Printf("[REDIS] Broadcast message sent to hub successfully")
	}

	log.Println("[REDIS] Redis pub/sub channel closed")
}
