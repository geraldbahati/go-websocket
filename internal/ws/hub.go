package ws

import (
	"go-websocket/internal/models"
	"hash/fnv"
	"log/slog"
	"sync"
)

const numBuckets = 32

// RedisPublisher defines the interface for publishing events to Redis
type RedisPublisher interface {
	PublishPresenceJoin(channelId, userId, userName string) error
	PublishPresenceLeave(channelId, userId string) error
	PublishTypingStart(channelId, userId, userName string) error
	PublishTypingStop(channelId, userId string) error
}

type bucket struct {
	sync.RWMutex
	channels map[string]map[*Client]bool
}

// Hub maintains active WebSocket connections and broadcasts messages
type Hub struct {
	buckets [numBuckets]*bucket

	// Register requests from clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Broadcast messages to clients in a channel (exported for Redis pubsub access)
	Broadcast chan *models.BroadcastMessage

	// Redis client for publishing events
	redisClient RedisPublisher
}

func NewHub(redisClient RedisPublisher) *Hub {
	h := &Hub{
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		Broadcast:   make(chan *models.BroadcastMessage),
		redisClient: redisClient,
	}

	for i := 0; i < numBuckets; i++ {
		h.buckets[i] = &bucket{
			channels: make(map[string]map[*Client]bool),
		}
	}

	return h
}

func (h *Hub) getBucket(channelId string) *bucket {
	hash := fnv.New32a()
	hash.Write([]byte(channelId))
	return h.buckets[hash.Sum32()%numBuckets]
}

func (h *Hub) Run() {
	slog.Info("[HUB] Starting hub event loop", "buckets", numBuckets)
	for {
		select {
		case client := <-h.register:
			slog.Debug("[HUB] Received register request", "user", client.userId, "channel", client.channelId)
			h.registerClient(client)

		case client := <-h.unregister:
			slog.Debug("[HUB] Received unregister request", "user", client.userId, "channel", client.channelId)
			h.unregisterClient(client)

		case message := <-h.Broadcast:
			// Process broadcast in a separate goroutine to avoid blocking the main loop
			// or just process it here if it's fast enough.
			// With sharding, we can potentially parallelize this if needed,
			// but for now, let's keep it simple but safe.
			h.broadcastToChannel(message)
		}
	}
}

func (h *Hub) registerClient(client *Client) {
	b := h.getBucket(client.channelId)
	b.Lock()

	if b.channels[client.channelId] == nil {
		slog.Debug("[HUB] Creating new channel", "channel", client.channelId)
		b.channels[client.channelId] = make(map[*Client]bool)
	}
	b.channels[client.channelId][client] = true

	clientCount := len(b.channels[client.channelId])
	slog.Info("[HUB] Client registered", "user", client.userId, "channel", client.channelId, "clientCount", clientCount)

	// Release lock before publishing to Redis
	b.Unlock()

	// Publish presence:join event synchronously (but outside the lock)
	// This ensures strict ordering for the same client without holding the lock during network I/O
	if err := h.redisClient.PublishPresenceJoin(client.channelId, client.userId, client.userName); err != nil {
		slog.Error("[HUB] Error publishing presence:join event", "error", err, "channel", client.channelId)
	} else {
		slog.Debug("[HUB] Published presence:join event", "user", client.userId, "channel", client.channelId)
	}
}

func (h *Hub) unregisterClient(client *Client) {
	b := h.getBucket(client.channelId)
	b.Lock()

	shouldPublishLeave := false
	if clients, ok := b.channels[client.channelId]; ok {
		if _, ok := clients[client]; ok {
			delete(clients, client)
			close(client.send)

			clientCount := len(clients)
			slog.Info("[HUB] Client unregistered", "user", client.userId, "channel", client.channelId, "clientCount", clientCount)

			// Clean up empty channels
			if clientCount == 0 {
				slog.Debug("[HUB] Channel is now empty, removing from hub", "channel", client.channelId)
				delete(b.channels, client.channelId)
			}

			shouldPublishLeave = true
		}
	}

	// Release lock before publishing to Redis
	b.Unlock()

	// Publish presence:leave event synchronously (but outside the lock)
	// This ensures strict ordering for the same client without holding the lock during network I/O
	if shouldPublishLeave {
		if err := h.redisClient.PublishPresenceLeave(client.channelId, client.userId); err != nil {
			slog.Error("[HUB] Error publishing presence:leave event", "error", err, "channel", client.channelId)
		} else {
			slog.Debug("[HUB] Published presence:leave event", "user", client.userId, "channel", client.channelId)
		}
	}
}

func (h *Hub) broadcastToChannel(message *models.BroadcastMessage) {
	b := h.getBucket(message.ChannelId)
	b.RLock()
	defer b.RUnlock()

	if clients, ok := b.channels[message.ChannelId]; ok {
		// clientCount := len(clients)
		// slog.Debug("[HUB] Broadcasting to channel", "channel", message.ChannelId, "clientCount", clientCount)

		sentCount := 0
		failedCount := 0

		for client := range clients {
			select {
			case client.send <- message.Payload:
				sentCount++
			default:
				// Client buffer full, disconnect
				slog.Warn("[HUB] Client buffer full, disconnecting", "user", client.userId, "channel", client.channelId)
				close(client.send)
				delete(clients, client)
				failedCount++
			}
		}

		// slog.Debug("[HUB] Broadcast complete", "channel", message.ChannelId, "sent", sentCount, "failed", failedCount)
	} else {
		// slog.Debug("[HUB] No clients connected to channel", "channel", message.ChannelId)
	}
}

// GetChannelUsers returns list of connected users in a channel
func (h *Hub) GetChannelUsers(channelId string) []string {
	b := h.getBucket(channelId)
	b.RLock()
	defer b.RUnlock()

	users := []string{}
	if clients, ok := b.channels[channelId]; ok {
		for client := range clients {
			users = append(users, client.userId)
		}
	}
	return users
}
