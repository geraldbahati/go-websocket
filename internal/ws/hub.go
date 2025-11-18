package ws

import (
	"go-websocket/internal/models"
	"log"
	"sync"
)

// RedisPublisher defines the interface for publishing events to Redis
type RedisPublisher interface {
	PublishPresenceJoin(channelId, userId, userName string) error
	PublishPresenceLeave(channelId, userId string) error
	PublishTypingStart(channelId, userId, userName string) error
	PublishTypingStop(channelId, userId string) error
}

// Hub maintains active WebSocket connections and broadcasts messages
type Hub struct {
	// Registered clients by channel ID
	// Map: channelId -> Set of clients
	channels map[string]map[*Client]bool

	// Lock for thread-safe access
	mu sync.RWMutex

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
	return &Hub{
		channels:    make(map[string]map[*Client]bool),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		Broadcast:   make(chan *models.BroadcastMessage),
		redisClient: redisClient,
	}
}

func (h *Hub) Run() {
	log.Println("[HUB] Starting hub event loop")
	for {
		select {
		case client := <-h.register:
			log.Printf("[HUB] Received register request for client (user: %s, channel: %s)", client.userId, client.channelId)
			h.registerClient(client)

		case client := <-h.unregister:
			log.Printf("[HUB] Received unregister request for client (user: %s, channel: %s)", client.userId, client.channelId)
			h.unregisterClient(client)

		case message := <-h.Broadcast:
			log.Printf("[HUB] Received broadcast message for channel: %s (payload size: %d bytes)", message.ChannelId, len(message.Payload))
			h.broadcastToChannel(message)
		}
	}
}

func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.channels[client.channelId] == nil {
		log.Printf("[HUB] Creating new channel: %s", client.channelId)
		h.channels[client.channelId] = make(map[*Client]bool)
	}
	h.channels[client.channelId][client] = true

	clientCount := len(h.channels[client.channelId])
	log.Printf("[HUB] Client registered (user: %s, userName: %s, channel: %s). Channel now has %d client(s)",
		client.userId, client.userName, client.channelId, clientCount)

	// Emit presence:join event
	if err := h.redisClient.PublishPresenceJoin(client.channelId, client.userId, client.userName); err != nil {
		log.Printf("[HUB] Error publishing presence:join event: %v", err)
	} else {
		log.Printf("[HUB] Published presence:join event (user: %s, channel: %s)", client.userId, client.channelId)
	}
}

func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.channels[client.channelId]; ok {
		if _, ok := clients[client]; ok {
			delete(clients, client)
			close(client.send)

			clientCount := len(clients)
			log.Printf("[HUB] Client unregistered (user: %s, channel: %s). Channel now has %d client(s)",
				client.userId, client.channelId, clientCount)

			// Clean up empty channels
			if clientCount == 0 {
				log.Printf("[HUB] Channel %s is now empty, removing from hub", client.channelId)
				delete(h.channels, client.channelId)
			}

			// Emit presence:leave event
			if err := h.redisClient.PublishPresenceLeave(client.channelId, client.userId); err != nil {
				log.Printf("[HUB] Error publishing presence:leave event: %v", err)
			} else {
				log.Printf("[HUB] Published presence:leave event (user: %s, channel: %s)", client.userId, client.channelId)
			}
		}
	} else {
		log.Printf("[HUB] Warning: Attempted to unregister client from non-existent channel: %s", client.channelId)
	}
}

func (h *Hub) broadcastToChannel(message *models.BroadcastMessage) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if clients, ok := h.channels[message.ChannelId]; ok {
		clientCount := len(clients)
		log.Printf("[HUB] Broadcasting to channel %s (%d client(s))", message.ChannelId, clientCount)

		sentCount := 0
		failedCount := 0

		for client := range clients {
			select {
			case client.send <- message.Payload:
				sentCount++
			default:
				// Client buffer full, disconnect
				log.Printf("[HUB] Client buffer full, disconnecting (user: %s, channel: %s)", client.userId, client.channelId)
				close(client.send)
				delete(clients, client)
				failedCount++
			}
		}

		log.Printf("[HUB] Broadcast complete for channel %s: sent=%d, failed=%d", message.ChannelId, sentCount, failedCount)
	} else {
		log.Printf("[HUB] No clients connected to channel: %s", message.ChannelId)
	}
}

// GetChannelUsers returns list of connected users in a channel
func (h *Hub) GetChannelUsers(channelId string) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	users := []string{}
	if clients, ok := h.channels[channelId]; ok {
		for client := range clients {
			users = append(users, client.userId)
		}
	}
	log.Printf("[HUB] GetChannelUsers for channel %s: %d user(s)", channelId, len(users))
	return users
}
