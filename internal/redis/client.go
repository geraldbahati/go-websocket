package redis

import (
	"context"
	"go-websocket/internal/models"
	"log/slog"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/goccy/go-json"
)

type Client struct {
	rdb *redis.Client
	ctx context.Context
}

func NewClient(redisURL string) *Client {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		slog.Error("Failed to parse Redis URL", "error", err)
		panic(err)
	}

	rdb := redis.NewClient(opt)
	ctx := context.Background()

	// Test connection
	if err := rdb.Ping(ctx).Err(); err != nil {
		slog.Error("Failed to connect to Redis", "error", err)
		panic(err)
	}

	slog.Info("Connected to Redis")

	return &Client{
		rdb: rdb,
		ctx: ctx,
	}
}

func (c *Client) Close() error {
	return c.rdb.Close()
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

func (c *Client) PublishTypingStart(channelId, userId, userName string, threadId *string) error {
	data := map[string]interface{}{
		"userId":   userId,
		"userName": userName,
	}

	// Include threadId if provided
	if threadId != nil && *threadId != "" {
		data["threadId"] = *threadId
	}

	event := models.Event{
		Type:      "typing:start",
		ChannelId: channelId,
		Timestamp: time.Now().Unix(),
		Data:      data,
	}

	return c.publishEvent(channelId, event)
}

func (c *Client) PublishTypingStop(channelId, userId string, threadId *string) error {
	data := map[string]interface{}{
		"userId": userId,
	}

	// Include threadId if provided
	if threadId != nil && *threadId != "" {
		data["threadId"] = *threadId
	}

	event := models.Event{
		Type:      "typing:stop",
		ChannelId: channelId,
		Timestamp: time.Now().Unix(),
		Data:      data,
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
		slog.Error("[REDIS] Failed to marshal event", "type", event.Type, "channel", channelId, "error", err)
		return err
	}

	channel := "channel:" + channelId
	result := c.rdb.Publish(c.ctx, channel, payload)
	if err := result.Err(); err != nil {
		slog.Error("[REDIS] Failed to publish event", "type", event.Type, "channel", channel, "error", err)
		return err
	}

	return nil
}
