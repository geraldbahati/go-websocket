package ws

import (
	"go-websocket/internal/models"
	"hash/fnv"
	"log/slog"
	"sync"
)

const numBuckets = 32

type RedisPublisher interface {
	PublishPresenceJoin(channelId, userId, userName string) error
	PublishPresenceLeave(channelId, userId string) error
	PublishTypingStart(channelId, userId, userName string, threadId *string) error
	PublishTypingStop(channelId, userId string, threadId *string) error
}

type bucket struct {
	sync.RWMutex
	channels  map[string]map[*Client]bool
	broadcast chan *models.BroadcastMessage
}

type Hub struct {
	buckets     [numBuckets]*bucket
	register    chan *Client
	unregister  chan *Client
	Broadcast   chan *models.BroadcastMessage
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
			channels:  make(map[string]map[*Client]bool),
			broadcast: make(chan *models.BroadcastMessage, 256),
		}
		go h.runBucketWorker(i)
	}

	return h
}

func (h *Hub) getBucket(channelId string) *bucket {
	hash := fnv.New32a()
	hash.Write([]byte(channelId))
	return h.buckets[hash.Sum32()%numBuckets]
}

func (h *Hub) Run() {
	slog.Info("[HUB] Started event loop", "buckets", numBuckets)
	for {
		select {
		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case message := <-h.Broadcast:
			b := h.getBucket(message.ChannelId)
			select {
			case b.broadcast <- message:
			default:
				slog.Warn("[HUB] Broadcast channel full, dropping message", "channel", message.ChannelId)
			}
		}
	}
}

func (h *Hub) runBucketWorker(bucketIndex int) {
	slog.Info("[HUB] Starting bucket worker", "bucket", bucketIndex)
	b := h.buckets[bucketIndex]

	for message := range b.broadcast {
		h.broadcastToChannel(message)
	}

	slog.Info("[HUB] Bucket worker stopped", "bucket", bucketIndex)
}

func (h *Hub) registerClient(client *Client) {
	b := h.getBucket(client.channelId)
	b.Lock()

	if b.channels[client.channelId] == nil {
		b.channels[client.channelId] = make(map[*Client]bool)
	}
	b.channels[client.channelId][client] = true

	clientCount := len(b.channels[client.channelId])
	slog.Info("[HUB] Client registered", "user", client.userId, "channel", client.channelId, "clients", clientCount)

	b.Unlock()

	if err := h.redisClient.PublishPresenceJoin(client.channelId, client.userId, client.userName); err != nil {
		slog.Error("[HUB] Failed to publish presence:join", "user", client.userId, "channel", client.channelId, "error", err)
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
			slog.Info("[HUB] Client unregistered", "user", client.userId, "channel", client.channelId, "clients", clientCount)

			if clientCount == 0 {
				delete(b.channels, client.channelId)
			}

			shouldPublishLeave = true
		}
	}

	b.Unlock()

	if shouldPublishLeave {
		if err := h.redisClient.PublishPresenceLeave(client.channelId, client.userId); err != nil {
			slog.Error("[HUB] Failed to publish presence:leave", "user", client.userId, "channel", client.channelId, "error", err)
		}
	}
}

func (h *Hub) broadcastToChannel(message *models.BroadcastMessage) {
	b := h.getBucket(message.ChannelId)
	b.RLock()
	defer b.RUnlock()

	if clients, ok := b.channels[message.ChannelId]; ok {
		for client := range clients {
			select {
			case client.send <- message.Payload:
			default:
				slog.Warn("[HUB] Client buffer full, disconnecting", "user", client.userId, "channel", client.channelId)
				close(client.send)
				delete(clients, client)
			}
		}
	}
}

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
