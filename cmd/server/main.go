package main

import (
	"context"
	"go-websocket/internal/auth"
	"go-websocket/internal/config"
	"go-websocket/internal/logger"
	"go-websocket/internal/redis"
	"go-websocket/internal/ws"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize Logger
	logger.Init(cfg.LogLevel)

	// Initial Kinde JWKs
	if err := auth.InitJWKS(cfg.KindeIssuerURL); err != nil {
		slog.Error("Failed to initialize JWKS", "error", err)
		os.Exit(1)
	}

	// Initialize Redis
	redisClient := redis.NewClient(cfg.RedisURL)
	defer redisClient.Close()

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

	server := &http.Server{
		Addr: ":" + cfg.Port,
	}

	// Start server in a goroutine
	go func() {
		slog.Info("WebSocket server starting", "port", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
	}

	slog.Info("Server exited")
}
