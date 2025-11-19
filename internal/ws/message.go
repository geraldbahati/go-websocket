package ws

import (
	"go-websocket/internal/auth"
	"log/slog"
	"net/http"
)

func ServeWS(hub *Hub, w http.ResponseWriter, r *http.Request) {
	remoteAddr := r.RemoteAddr
	slog.Debug("[WS] New WebSocket connection request", "from", remoteAddr)

	// Extract JWT token from query param or header
	token := r.URL.Query().Get("token")
	if token == "" {
		token = r.Header.Get("Authorization")
		slog.Debug("[WS] Token from Authorization header", "from", remoteAddr)
	} else {
		slog.Debug("[WS] Token from query parameter", "from", remoteAddr)
	}

	if token == "" {
		slog.Warn("[WS] No token provided", "from", remoteAddr)
		http.Error(w, "Unauthorized: token required", http.StatusUnauthorized)
		return
	}

	// Validate Kinde JWT token
	claims, err := auth.ValidateToken(token)
	if err != nil {
		slog.Warn("[WS] Token validation failed", "from", remoteAddr, "error", err)
		http.Error(w, "Unauthorized: invalid token", http.StatusUnauthorized)
		return
	}

	slog.Info("[WS] Token validated successfully", "user", claims.Subject, "email", claims.Email, "from", remoteAddr)

	// Extract channel ID from query
	channelId := r.URL.Query().Get("channelId")
	if channelId == "" {
		slog.Warn("[WS] No channelId provided", "user", claims.Subject, "from", remoteAddr)
		http.Error(w, "channelId required", http.StatusBadRequest)
		return
	}

	slog.Debug("[WS] Attempting to join channel", "channel", channelId, "user", claims.Subject, "userName", claims.GivenName)

	// TODO: Verify user has access to this channel
	// Could call Next.js API or query Postgres directly

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("[WS] Failed to upgrade connection", "user", claims.Subject, "channel", channelId, "error", err)
		return
	}

	slog.Info("[WS] Connection upgraded successfully", "user", claims.Subject, "channel", channelId)

	client := &Client{
		hub:       hub,
		conn:      conn,
		send:      make(chan []byte, 256),
		channelId: channelId,
		userId:    claims.Subject,
		userName:  claims.GivenName,
	}

	slog.Debug("[WS] Client created, sending register request", "user", client.userId, "channel", client.channelId)
	client.hub.register <- client

	// Start goroutines for read/write
	slog.Debug("[WS] Starting WritePump and ReadPump goroutines", "user", client.userId, "channel", client.channelId)
	go client.WritePump()
	go client.ReadPump()
}
