// Package redis provides the Queue interface implementation backed by Redis
// Streams (ADR-003). This is the only package in the codebase that imports a
// Redis client — everything above depends on provider.Queue. Swapping the
// backbone (Kafka, NATS) touches only this file.
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"archadilm/internal/domain/provider"
)

// Queue wraps a Redis client and implements provider.Queue using Redis Streams
// with consumer-group semantics (XADD / XREADGROUP / XACK).
type Queue struct {
	client *goredis.Client
}

// NewQueue dials Redis using the given URL (e.g. redis://localhost:6379) and
// returns a Queue or an error if the connection is unreachable.
func NewQueue(url string) (*Queue, error) {
	opts, err := goredis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("redis: parse url: %w", err)
	}
	c := goredis.NewClient(opts)
	if err := c.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis: ping: %w", err)
	}
	return &Queue{client: c}, nil
}

// Close releases the underlying connection pool.
func (q *Queue) Close() error { return q.client.Close() }

// Ping checks that Redis is reachable. Used by the API's /healthz check.
func (q *Queue) Ping(ctx context.Context) error {
	return q.client.Ping(ctx).Err()
}

// Client returns the underlying go-redis client, for callers that need to
// build another Redis-backed component (e.g. JobStore) against the same
// connection rather than opening a second one.
func (q *Queue) Client() *goredis.Client {
	return q.client
}

// FetchStreamEntry reads a single message from `stream` by its Redis Stream
// entry ID (e.g. "1700000000000-0") and decodes it back into an Event. Used
// by DLQ replay to look up one dead-lettered entry without consuming the
// whole stream.
func (q *Queue) FetchStreamEntry(ctx context.Context, stream, id string) (provider.Event, error) {
	msgs, err := q.client.XRange(ctx, stream, id, id).Result()
	if err != nil {
		return provider.Event{}, fmt.Errorf("redis: fetch entry %s from %s: %w", id, stream, err)
	}
	if len(msgs) == 0 {
		return provider.Event{}, fmt.Errorf("redis: entry %s not found in %s", id, stream)
	}

	name, _ := msgs[0].Values["name"].(string)
	payloadStr, _ := msgs[0].Values["payload"].(string)
	traceID, _ := msgs[0].Values["trace_id"].(string)

	var payload map[string]any
	_ = json.Unmarshal([]byte(payloadStr), &payload)

	return provider.Event{Name: name, Payload: payload, TraceID: traceID}, nil
}

// DeleteStreamEntry removes a single message from `stream` by ID (XDEL).
// Used to remove a DLQ entry once it has been successfully replayed.
func (q *Queue) DeleteStreamEntry(ctx context.Context, stream, id string) error {
	return q.client.XDel(ctx, stream, id).Err()
}

// Publish serialises an Event and appends it to the named Redis Stream using
// XADD. Fields: name, payload (JSON), trace_id.
func (q *Queue) Publish(ctx context.Context, stream string, e provider.Event) error {
	payload, err := json.Marshal(e.Payload)
	if err != nil {
		return fmt.Errorf("redis: marshal payload: %w", err)
	}
	return q.client.XAdd(ctx, &goredis.XAddArgs{
		Stream: stream,
		Values: map[string]interface{}{
			"name":     e.Name,
			"payload":  string(payload),
			"trace_id": e.TraceID,
		},
	}).Err()
}

// Consume creates the consumer group (if absent) and starts a goroutine that
// reads messages with XREADGROUP, forwarding them onto the returned channel.
// The caller must invoke QueuedEvent.Ack after successful processing to
// advance the consumer-group cursor (XACK). Closing ctx stops the goroutine
// and closes the channel.
func (q *Queue) Consume(ctx context.Context, stream, group string) (<-chan provider.QueuedEvent, error) {
	// MKSTREAM creates the stream if it doesn't exist yet; "0" starts from
	// the beginning for recovery; "$" would miss messages published before
	// the group was created. We use "0" so a restarted worker catches up.
	err := q.client.XGroupCreateMkStream(ctx, stream, group, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return nil, fmt.Errorf("redis: create group %s/%s: %w", stream, group, err)
	}

	ch := make(chan provider.QueuedEvent, 64)
	consumerName := fmt.Sprintf("%s-%d", group, time.Now().UnixNano())

	go func() {
		defer close(ch)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			streams, err := q.client.XReadGroup(ctx, &goredis.XReadGroupArgs{
				Group:    group,
				Consumer: consumerName,
				Streams:  []string{stream, ">"},
				Count:    10,
				Block:    2 * time.Second,
			}).Result()
			if err != nil {
				// redis.Nil == no messages within block timeout; not an error.
				if err == goredis.Nil || ctx.Err() != nil {
					continue
				}
				// Transient network error — log and retry.
				// In production the structured log would carry trace_id but
				// this package has no logger dependency by design.
				time.Sleep(500 * time.Millisecond)
				continue
			}

			for _, s := range streams {
				for _, msg := range s.Messages {
					msgID := msg.ID
					qe := q.decode(stream, group, msgID, msg.Values)
					select {
					case ch <- qe:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()
	return ch, nil
}

func (q *Queue) decode(stream, group, msgID string, vals map[string]interface{}) provider.QueuedEvent {
	name, _ := vals["name"].(string)
	payloadStr, _ := vals["payload"].(string)
	traceID, _ := vals["trace_id"].(string)

	var payload map[string]any
	_ = json.Unmarshal([]byte(payloadStr), &payload)

	return provider.QueuedEvent{
		Event: provider.Event{
			Name:    name,
			Payload: payload,
			TraceID: traceID,
		},
		Ack: func(ctx context.Context) error {
			return q.client.XAck(ctx, stream, group, msgID).Err()
		},
	}
}
