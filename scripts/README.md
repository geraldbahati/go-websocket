# Test Scripts

## test-redis-publish.sh

Tests the Redis pub/sub functionality by publishing various event types.

### Usage

```bash
# Test with default channel (test-channel)
./scripts/test-redis-publish.sh

# Test with specific channel
./scripts/test-redis-publish.sh my-channel-id
```

### What it tests

1. **message:created** - New message event
2. **typing:start** - User started typing
3. **typing:stop** - User stopped typing
4. **presence:join** - User joined channel
5. **presence:leave** - User left channel

### Viewing Results

After running the script, check the logs to see the message flow:

```bash
# View live logs
make docker-logs

# Or
docker-compose logs -f websocket-server
```

You should see logs like:
```
[REDIS] Received message from Redis (channel: channel:test-channel, payload size: X bytes)
[REDIS] Event parsed successfully (type: message:created, channelId: test-channel, timestamp: ...)
[REDIS] Sending broadcast message to hub (channelId: test-channel)
[HUB] Received broadcast message for channel: test-channel (payload size: X bytes)
[HUB] Broadcasting to channel test-channel (N client(s))
```
