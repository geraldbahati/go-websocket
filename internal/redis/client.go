package redis

import (
	"context"
	"encoding/json"
	"go-websocket/internal/models"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
)

type Client struct {
	rdb *redis.Client
	ctx context.Context
}

func NewClient(redisURL string) *Client {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatal(err)
	}

	rdb := redis.NewClient(opt)
	ctx := context.Background()

	// Test connection
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal("Failed to connect to Redis:", err)
	}

	log.Println("Connected to Redis")

	return &Client{
		rdb: rdb,
		ctx: ctx,
	}
}

// Publish events to Redis

func (c *Client) PublishMessageCreated(channelId string, message interface{}) error {
	event := models.Event{
		Type:      "message:created",
		ChannelId: channelId,
		Timestamp: time.Now().Unix(),
		Data:      message,
	}

	return c.publishEvent(channelId, event)
}

func (c *Client) PublishTypingStart(channelId, userId, userName string) error {
	event := models.Event{
		Type:      "typing:start",
		ChannelId: channelId,
		Timestamp: time.Now().Unix(),
		Data: map[string]string{
			"userId":   userId,
			"userName": userName,
		},
	}

	return c.publishEvent(channelId, event)
}

func (c *Client) PublishTypingStop(channelId, userId string) error {
	event := models.Event{
		Type:      "typing:stop",
		ChannelId: channelId,
		Timestamp: time.Now().Unix(),
		Data: map[string]string{
			"userId": userId,
		},
	}

	return c.publishEvent(channelId, event)
}

func (c *Client) PublishPresenceJoin(channelId, userId, userName string) error {
	event := models.Event{
		Type:      "presence:join",
		ChannelId: channelId,
		Timestamp: time.Now().Unix(),
		Data: map[string]string{
			"userId":   userId,
			"userName": userName,
		},
	}

	return c.publishEvent(channelId, event)
}

func (c *Client) PublishPresenceLeave(channelId, userId string) error {
	event := models.Event{
		Type:      "presence:leave",
		ChannelId: channelId,
		Timestamp: time.Now().Unix(),
		Data: map[string]string{
			"userId": userId,
		},
	}

	return c.publishEvent(channelId, event)
}

func (c *Client) publishEvent(channelId string, event models.Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		log.Printf("[REDIS] Error marshaling event (type: %s, channelId: %s): %v", event.Type, channelId, err)
		return err
	}

	// Publish to channel-specific Redis channel
	channel := "channel:" + channelId
	log.Printf("[REDIS] Publishing event (type: %s, channel: %s, payload size: %d bytes)", event.Type, channel, len(payload))

	result := c.rdb.Publish(c.ctx, channel, payload)
	if err := result.Err(); err != nil {
		log.Printf("[REDIS] Error publishing event (type: %s, channel: %s): %v", event.Type, channel, err)
		return err
	}

	subscribers := result.Val()
	log.Printf("[REDIS] Event published successfully (type: %s, channel: %s, subscribers: %d)", event.Type, channel, subscribers)

	return nil
}
