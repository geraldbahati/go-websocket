# Go WebSocket Server

A scalable real-time WebSocket server built with Go, Redis pub/sub, and Kinde authentication.

## Features

- **WebSocket Communication**: Real-time bidirectional messaging
- **Redis Pub/Sub**: Horizontal scaling across multiple server instances
- **JWT Authentication**: Secure authentication using Kinde
- **Channel-based Broadcasting**: Targeted message delivery to specific channels
- **Presence Tracking**: Real-time user join/leave notifications
- **Typing Indicators**: Live typing status updates

## Quick Start

### Prerequisites

- Docker and Docker Compose (recommended)
- OR Go 1.25+ and Redis (for local development)
- Kinde account for authentication

### Using Docker (Recommended)

1. **Clone and setup environment**:

   ```bash
   cp .env.example .env
   # Edit .env and add your KINDE_ISSUER_URL
   ```

2. **Start all services**:

   ```bash
   docker-compose up -d
   ```

3. **View logs**:

   ```bash
   docker-compose logs -f websocket-server
   ```

4. **Test the server**:
   ```bash
   curl http://localhost:8080/health
   # Should return: OK
   ```

### Local Development

1. **Start Redis only**:

   ```bash
   docker-compose -f docker-compose.dev.yml up -d
   ```

2. **Set environment variables**:

   ```bash
   export KINDE_ISSUER_URL=https://your-subdomain.kinde.com
   export REDIS_URL=redis://localhost:6379
   export PORT=8080
   ```

3. **Run the server**:
   ```bash
   go run cmd/server/main.go
   ```

## WebSocket Connection

Connect to the WebSocket server at:

```
ws://localhost:8080/ws?token={JWT_TOKEN}&channelId={CHANNEL_ID}
```

**Parameters:**

- `token`: Kinde JWT access token (can also be sent in `Authorization` header)
- `channelId`: Channel/room identifier to join

**Example using JavaScript**:

```javascript
const token = "your_kinde_jwt_token";
const channelId = "channel_123";
const ws = new WebSocket(
  `ws://localhost:8080/ws?token=${token}&channelId=${channelId}`
);

ws.onopen = () => console.log("Connected");
ws.onmessage = (event) => console.log("Message:", event.data);
```

## Event Types

### Server → Client Events

All events follow this structure:

```json
{
  "type": "event_type",
  "channelId": "channel_id",
  "timestamp": 1234567890,
  "data": {
    /* event-specific data */
  }
}
```

**Event Types:**

- `message:created` - New message in channel
- `message:updated` - Message updated in channel
- `message:deleted` - Message deleted from channel
- `typing:start` - User started typing
- `typing:stop` - User stopped typing
- `presence:join` - User joined channel
- `presence:leave` - User left channel

### Client → Server Events

**Typing Indicator:**

```json
{
  "type": "typing:start",
  "channelId": "channel_id"
}
```

```json
{
  "type": "typing:stop",
  "channelId": "channel_id"
}
```

## Publishing Events via Redis

External services (like your Next.js API) can publish events to Redis:

```bash
# Publish a message to channel:123
redis-cli PUBLISH "channel:123" '{
  "type": "message:created",
  "channelId": "123",
  "timestamp": 1234567890,
  "data": {
    "id": "msg_1",
    "content": "Hello world",
    "authorId": "user_1",
    "authorName": "John Doe"
  }
}'
```

## Architecture

```
┌─────────────────┐        ┌──────────┐        ┌─────────────────┐
│   WebSocket     │◄──────►│   Hub    │◄──────►│  Redis Pub/Sub  │
│   Clients       │        │          │        │                 │
└─────────────────┘        └──────────┘        └─────────────────┘
                                │                        ▲
                                │                        │
                                ▼                        │
                           ┌─────────┐                   │
                           │  Redis  │───────────────────┘
                           │ Publish │
                           └─────────┘
```

See [CLAUDE.md](./CLAUDE.md) for detailed architecture documentation.

## Docker Services

### Production (`docker-compose.yml`)

- **websocket-server**: Go WebSocket server
- **redis**: Redis 7 with persistence

### Development (`docker-compose.dev.yml`)

- **redis**: Redis 7 for local development
- **redis-commander**: Web UI for Redis debugging (http://localhost:8081)

## Environment Variables

| Variable           | Description           | Required | Default                  |
| ------------------ | --------------------- | -------- | ------------------------ |
| `KINDE_ISSUER_URL` | Your Kinde issuer URL | Yes      | -                        |
| `REDIS_URL`        | Redis connection URL  | Yes      | `redis://localhost:6379` |
| `PORT`             | Server port           | No       | `8080`                   |

## Health Check

The server exposes a health check endpoint:

```bash
GET /health
# Returns: OK (200 status)
```

## Development

```bash
# Run tests
go test ./...

# Format code
go fmt ./...

# Vet code
go vet ./...

# Build binary
go build -o bin/websocket-server cmd/server/main.go

# Run with live reload (requires air)
air
```

## Production Deployment

1. Set environment variables in your hosting platform
2. Use `docker-compose.yml` for containerized deployment
3. Ensure Redis is accessible from all server instances
4. Configure load balancer for WebSocket sticky sessions (or use Redis pub/sub for scaling)

## Security Notes

- JWT tokens are validated against Kinde's JWKS
- JWKS is cached and refreshed every 24 hours
- WebSocket origin validation is disabled in development (update `client.go:30` for production)
- Channel access authorization should be implemented (see `message.go:30`)

## License

MIT
