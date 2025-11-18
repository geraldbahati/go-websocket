#!/bin/bash

# Test Redis pub/sub by publishing messages
# This script helps test and debug the WebSocket server

echo "🧪 Testing Redis Pub/Sub"
echo "========================"
echo ""

CHANNEL_ID=${1:-"test-channel"}

echo "Publishing to channel: $CHANNEL_ID"
echo ""

# Test 1: Message Created Event
echo "📨 Test 1: Publishing message:created event..."
docker exec go-websocket-redis redis-cli PUBLISH "channel:$CHANNEL_ID" '{
  "type": "message:created",
  "channelId": "'$CHANNEL_ID'",
  "timestamp": '$(date +%s)',
  "data": {
    "id": "msg_123",
    "content": "Hello from Redis test!",
    "authorId": "user_test",
    "authorName": "Test User",
    "authorEmail": "test@example.com",
    "authorAvatar": "https://example.com/avatar.jpg",
    "createdAt": "'$(date -u +"%Y-%m-%dT%H:%M:%SZ")'"
  }
}'

echo ""
sleep 1

# Test 2: Typing Start Event
echo "⌨️  Test 2: Publishing typing:start event..."
docker exec go-websocket-redis redis-cli PUBLISH "channel:$CHANNEL_ID" '{
  "type": "typing:start",
  "channelId": "'$CHANNEL_ID'",
  "timestamp": '$(date +%s)',
  "data": {
    "userId": "user_test",
    "userName": "Test User"
  }
}'

echo ""
sleep 1

# Test 3: Typing Stop Event
echo "🛑 Test 3: Publishing typing:stop event..."
docker exec go-websocket-redis redis-cli PUBLISH "channel:$CHANNEL_ID" '{
  "type": "typing:stop",
  "channelId": "'$CHANNEL_ID'",
  "timestamp": '$(date +%s)',
  "data": {
    "userId": "user_test"
  }
}'

echo ""
sleep 1

# Test 4: Presence Join Event
echo "👋 Test 4: Publishing presence:join event..."
docker exec go-websocket-redis redis-cli PUBLISH "channel:$CHANNEL_ID" '{
  "type": "presence:join",
  "channelId": "'$CHANNEL_ID'",
  "timestamp": '$(date +%s)',
  "data": {
    "userId": "user_new",
    "userName": "New User",
    "userAvatar": "https://example.com/new-avatar.jpg"
  }
}'

echo ""
sleep 1

# Test 5: Presence Leave Event
echo "👋 Test 5: Publishing presence:leave event..."
docker exec go-websocket-redis redis-cli PUBLISH "channel:$CHANNEL_ID" '{
  "type": "presence:leave",
  "channelId": "'$CHANNEL_ID'",
  "timestamp": '$(date +%s)',
  "data": {
    "userId": "user_old"
  }
}'

echo ""
echo "✅ All test messages published!"
echo ""
echo "📋 Check logs with: make docker-logs"
echo "   or: docker-compose logs -f websocket-server"
