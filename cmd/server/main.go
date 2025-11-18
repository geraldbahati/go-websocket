package main

import (
	"go-websocket/internal/auth"
	"go-websocket/internal/redis"
	"go-websocket/internal/ws"
	"log"
	"net/http"
	"os"
)

func main() {
	// Initial Kinde JWKs
	kindeIssuerURL := os.Getenv("KINDE_ISSUER_URL")
	if err := auth.InitJWKS(kindeIssuerURL); err != nil {
		log.Fatal("Failed to initialize JWKS:", err)
	}

	// Initialize Redis
	redisClient := redis.NewClient(os.Getenv("REDIS_URL"))

	// Create hub
	hub := ws.NewHub(redisClient)
	go hub.Run()

	// Subscribe to Redis
	go redis.SubscribeToEvents(redisClient, hub)

	// Routes
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		ws.ServeWS(hub, w, r)
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("WebSocket server starting on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
