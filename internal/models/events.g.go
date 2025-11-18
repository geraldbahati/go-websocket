package models

type Event struct {
	Type      string      `json:"type"`
	ChannelId string      `json:"channelId"`
	Timestamp int64       `json:"timestamp"`
	Data      interface{} `json:"data"`
}

type BroadcastMessage struct {
	ChannelId string
	Payload   []byte
}

// Specific event data structures

type MessageCreatedData struct {
	ID           string `json:"id"`
	Content      string `json:"content"`
	ImageUrl     string `json:"imageUrl,omitempty"`
	AuthorId     string `json:"authorId"`
	AuthorName   string `json:"authorName"`
	AuthorEmail  string `json:"authorEmail"`
	AuthorAvatar string `json:"authorAvatar"`
	CreatedAt    string `json:"createdAt"`
}

type TypingData struct {
	UserId   string `json:"userId"`
	UserName string `json:"userName"`
}

type PresenceData struct {
	UserId     string `json:"userId"`
	UserName   string `json:"userName"`
	UserAvatar string `json:"userAvatar,omitempty"`
}
