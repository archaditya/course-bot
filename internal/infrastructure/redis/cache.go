// Package redis — cache.go provides a thin response cache for the chat
// pipeline. Keyed by conversation + query hash, with a short TTL so
// repeated identical questions skip the full RAG pipeline entirely.
// Uses the existing go-redis client — no new infrastructure needed.
package redis

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Cache wraps a go-redis client and provides conversation-scoped caching
// for chat responses. All keys are prefixed with `chat:v1:` and scoped to
// a conversation ID.
type Cache struct {
	client *goredis.Client
}

// NewCache creates a Cache backed by the given go-redis client (reused
// from the queue connection — no second Redis connection needed).
func NewCache(client *goredis.Client) *Cache {
	return &Cache{client: client}
}

// key builds a deterministic cache key from conversation ID and user query.
func key(conversationID, userContent string) string {
	h := sha256.Sum256([]byte(userContent))
	return fmt.Sprintf("chat:v1:%s:%x", conversationID, h[:8])
}

// Get retrieves a cached response. Returns ("", false) on miss.
func (c *Cache) Get(ctx context.Context, conversationID, userContent string) (string, bool) {
	val, err := c.client.Get(ctx, key(conversationID, userContent)).Result()
	if err != nil {
		return "", false
	}
	return val, true
}

// Set stores a response in the cache with the given TTL.
func (c *Cache) Set(ctx context.Context, conversationID, userContent, value string, ttl time.Duration) {
	_ = c.client.Set(ctx, key(conversationID, userContent), value, ttl).Err()
}

// InvalidateConversation removes all cached responses for a conversation.
// Called when sources are added or deleted so stale answers aren't served.
func (c *Cache) InvalidateConversation(ctx context.Context, conversationID string) {
	pattern := fmt.Sprintf("chat:v1:%s:*", conversationID)
	iter := c.client.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		_ = c.client.Del(ctx, iter.Val()).Err()
	}
}
