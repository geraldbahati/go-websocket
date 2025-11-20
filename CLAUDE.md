# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a **Go WebSocket server** that provides real-time communication for a chat application. It uses Redis pub/sub for horizontal scaling and Kinde for JWT-based authentication.

## Key Commands

### Using Makefile (Easiest)
```bash
# View all available commands
make help

# Start all services with Docker
make docker-up

# View logs
make docker-logs

# Stop services
make docker-down

# Rebuild and restart
make docker-rebuild

# Start only Redis for local development
make dev-up

# Build the binary
make build

# Run tests
make test

# Format and vet code
make lint
```

### Running with Docker (Alternative)
```bash
# Start all services (Redis + WebSocket server)
docker-compose up -d

# View logs
docker-compose logs -f websocket-server

# Stop all services
docker-compose down

# Rebuild and restart
docker-compose up -d --build

# Start only Redis for local development
docker-compose -f docker-compose.dev.yml up -d

# Access Redis Commander (debugging tool)
# Available at http://localhost:8081 when using docker-compose.dev.yml
```

### Running Locally (Without Docker)
```bash
# Ensure Redis is running locally first
# Then set environment variables and run:
go run cmd/server/main.go

# Or from root (if main.go is updated)
go run main.go
```

### Development
```bash
# Build the application
go build -o bin/websocket-server cmd/server/main.go

# Run tests (when available)
go test ./...

# Run specific package tests
go test ./internal/ws
go test ./internal/auth

# Format code
go fmt ./...

# Vet code for issues
go vet ./...

# Get dependencies
go mod tidy
go mod download
```

## Architecture

### Core Components

**Entry Point**: `cmd/server/main.go`
- Initializes Kinde JWKS for JWT validation
- Sets up Redis connection
- Creates and starts the Hub goroutine
- Establishes Redis pub/sub subscription
- Defines HTTP routes (`/ws` and `/health`)

### Package Structure

**`internal/ws`** - WebSocket connection management
- **Hub**: Central coordinator that manages all active WebSocket connections. Uses channels to handle concurrent operations (register, unregister, broadcast). Organizes clients by channelId for targeted message delivery.
- **Client**: Represents a single WebSocket connection. Runs two goroutines: ReadPump (reads from WebSocket) and WritePump (writes to WebSocket with periodic pings).
- **ServeWS**: HTTP handler that upgrades connections to WebSocket. Validates JWT, extracts channelId, and creates Client instances.

**`internal/auth`** - Kinde JWT authentication
- Fetches and caches Kinde's JWKS (JSON Web Key Set) from `{KINDE_ISSUER_URL}/.well-known/jwks.json`
- Auto-refreshes JWKS every 24 hours
- ValidateToken verifies JWT signature, expiration, and issuer
- Extracts user claims (userId from Subject, userName from GivenName)

**`internal/redis`** - Redis pub/sub integration
- **Client**: Wrapper around go-redis client with publish methods for different event types
- **PubSub**: Subscribes to `channel:*` pattern and forwards events to Hub's broadcast channel
- Event types: `message:created`, `typing:start`, `typing:stop`, `presence:join`, `presence:leave`

**`internal/models`** - Shared data structures
- Event: Standard structure for all Redis-published events
- BroadcastMessage: Internal structure for Hub broadcast channel
- Typed data structures for message, typing, and presence events

### Data Flow

1. **External System → Redis**: Next.js API or other services publish events to Redis channels (`channel:{channelId}`)
2. **Redis → Hub**: PubSub goroutine receives events and sends to Hub's broadcast channel
3. **Hub → Clients**: Hub broadcasts messages to all clients subscribed to that channelId
4. **Client → Redis**: Client events (typing indicators) are published back to Redis for distribution

### Connection Flow

1. Client connects to `/ws?token={jwt}&channelId={id}`
2. JWT validated against Kinde JWKS
3. WebSocket upgraded, Client struct created
4. Client registered with Hub (mapped to channelId)
5. Presence:join event published to Redis
6. ReadPump and WritePump goroutines started
7. On disconnect: Client unregistered, presence:leave published

## Environment Variables

Required environment variables (set before running):

```bash
KINDE_ISSUER_URL  # e.g., https://your-subdomain.kinde.com
REDIS_URL         # e.g., redis://localhost:6379 or Redis Cloud URL
PORT              # Optional, defaults to 8080
```

**Setup Instructions:**
1. Copy `.env.example` to `.env`
2. Update `KINDE_ISSUER_URL` with your Kinde domain
3. When using Docker Compose, `REDIS_URL` is automatically configured
4. For local development, ensure Redis is running and set `REDIS_URL=redis://localhost:6379`

## Important Implementation Details

### Concurrency Patterns
- Hub uses RWMutex for thread-safe access to channels map
- Channel-based communication for register/unregister/broadcast operations
- Each Client runs two independent goroutines (ReadPump/WritePump)

### WebSocket Configuration
- WriteWait: 10s
- PongWait: 60s
- PingPeriod: 54s (90% of PongWait)
- MaxMessageSize: 512KB
- Send buffer: 256 messages per client

### Redis Channel Naming
- Pattern: `channel:{channelId}`
- Subscription uses pattern matching: `channel:*`

### TODO Items in Code
- `internal/ws/client.go:30`: Validate origin in production (CheckOrigin currently returns true)
- `internal/ws/message.go:30`: Verify user has access to channel (needs authorization check)

## Testing Strategy

When adding tests, focus on:
- Hub concurrent operations (register/unregister/broadcast)
- JWT validation with mock JWKS
- Redis pub/sub message flow
- WebSocket message handling

## Adding New Event Types

1. Define event type constant and data structure in `internal/models/events.g.go`
2. Add publish method in `internal/redis/client.go`
3. Handle in `internal/ws/client.go` handleClientMessage if client-initiated
4. Ensure event flows through Redis → PubSub → Hub → Clients

## Debugging

- Hub logs when it registers/unregisters clients
- Redis connection logged on startup
- JWKS fetch/refresh logged with key count
- WebSocket errors logged in Client.ReadPump
