package ws

import (
	"go-websocket/internal/auth"
	"log"
	"net/http"
)

func ServeWS(hub *Hub, w http.ResponseWriter, r *http.Request) {
	remoteAddr := r.RemoteAddr
	log.Printf("[WS] New WebSocket connection request from %s", remoteAddr)

	// Extract JWT token from query param or header
	token := r.URL.Query().Get("token")
	if token == "" {
		token = r.Header.Get("Authorization")
		log.Printf("[WS] Token from Authorization header (%s)", remoteAddr)
	} else {
		log.Printf("[WS] Token from query parameter (%s)", remoteAddr)
	}

	if token == "" {
		log.Printf("[WS] No token provided (%s)", remoteAddr)
		http.Error(w, "Unauthorized: token required", http.StatusUnauthorized)
		return
	}

	// Validate Kinde JWT token
	claims, err := auth.ValidateToken(token)
	if err != nil {
		log.Printf("[WS] Token validation failed (%s): %v", remoteAddr, err)
		http.Error(w, "Unauthorized: invalid token", http.StatusUnauthorized)
		return
	}

	log.Printf("[WS] Token validated successfully (user: %s, email: %s, from: %s)", claims.Subject, claims.Email, remoteAddr)

	// Extract channel ID from query
	channelId := r.URL.Query().Get("channelId")
	if channelId == "" {
		log.Printf("[WS] No channelId provided (user: %s, from: %s)", claims.Subject, remoteAddr)
		http.Error(w, "channelId required", http.StatusBadRequest)
		return
	}

	log.Printf("[WS] Attempting to join channel: %s (user: %s, userName: %s)", channelId, claims.Subject, claims.GivenName)

	// TODO: Verify user has access to this channel
	// Could call Next.js API or query Postgres directly

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WS] Failed to upgrade connection (user: %s, channel: %s): %v", claims.Subject, channelId, err)
		return
	}

	log.Printf("[WS] Connection upgraded successfully (user: %s, channel: %s)", claims.Subject, channelId)

	client := &Client{
		hub:       hub,
		conn:      conn,
		send:      make(chan []byte, 256),
		channelId: channelId,
		userId:    claims.Subject,
		userName:  claims.GivenName,
	}

	log.Printf("[WS] Client created, sending register request (user: %s, channel: %s)", client.userId, client.channelId)
	client.hub.register <- client

	// Start goroutines for read/write
	log.Printf("[WS] Starting WritePump and ReadPump goroutines (user: %s, channel: %s)", client.userId, client.channelId)
	go client.WritePump()
	go client.ReadPump()
}
