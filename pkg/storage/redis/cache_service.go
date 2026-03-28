package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// CacheService wraps a Redis client that supports RedisJSON commands
// (JSON.SET, JSON.GET, JSON.NUMINCRBY). Requires Redis Stack or
// the RedisJSON module; plain Redis will error on these commands.
type CacheService struct {
	client *redis.Client
}

func NewCacheService(client *redis.Client) *CacheService {
	return &CacheService{client: client}
}

// SetJSON stores value as a JSON document at key with the given TTL.
// Internally executes: JSON.SET key $ <json>  +  EXPIRE key ttl.
func (c *CacheService) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	pipe := c.client.Pipeline()
	pipe.Do(ctx, "JSON.SET", key, "$", string(data))
	pipe.Expire(ctx, key, ttl)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("redis JSON.SET: %w", err)
	}
	return nil
}

// GetJSON reads the JSON document stored at key into dest.
// Returns redis.Nil when the key does not exist.
func (c *CacheService) GetJSON(ctx context.Context, key string, dest any) error {
	res, err := c.client.Do(ctx, "JSON.GET", key, "$").Result()
	if err != nil {
		return err
	}
	raw, ok := res.(string)
	if !ok {
		return fmt.Errorf("unexpected JSON.GET result type %T", res)
	}
	// JSON.GET with path "$" returns a JSON array wrapper, e.g. [{"code":"123456","retry":0}]
	var wrapper []json.RawMessage
	if err := json.Unmarshal([]byte(raw), &wrapper); err != nil {
		return fmt.Errorf("unmarshal JSON.GET wrapper: %w", err)
	}
	if len(wrapper) == 0 {
		return redis.Nil
	}
	if err := json.Unmarshal(wrapper[0], dest); err != nil {
		return fmt.Errorf("unmarshal JSON.GET value: %w", err)
	}
	return nil
}

// JSONIncrementField atomically increments a numeric field inside the
// JSON document at key. Returns the new value.
// Internally executes: JSON.NUMINCRBY key $.field increment.
func (c *CacheService) JSONIncrementField(ctx context.Context, key, field string, increment int64) (int64, error) {
	path := fmt.Sprintf("$.%s", field)
	res, err := c.client.Do(ctx, "JSON.NUMINCRBY", key, path, increment).Result()
	if err != nil {
		return 0, fmt.Errorf("redis JSON.NUMINCRBY: %w", err)
	}
	// Result is a JSON array like [3]
	raw, ok := res.(string)
	if !ok {
		return 0, fmt.Errorf("unexpected JSON.NUMINCRBY result type %T", res)
	}
	var vals []int64
	if err := json.Unmarshal([]byte(raw), &vals); err != nil {
		return 0, fmt.Errorf("unmarshal JSON.NUMINCRBY: %w", err)
	}
	if len(vals) == 0 {
		return 0, fmt.Errorf("empty JSON.NUMINCRBY result")
	}
	return vals[0], nil
}

// Delete removes a key from Redis.
func (c *CacheService) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

// SetNX sets a key only if it does not already exist (distributed lock).
// Returns true if the key was set, false if it already existed.
func (c *CacheService) SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	res, err := c.client.Do(ctx, "SET", key, value, "NX", "PX", ttl.Milliseconds()).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return res == "OK", nil
}

// Ping checks the Redis connection.
func (c *CacheService) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Client returns the underlying redis client for advanced usage.
func (c *CacheService) Client() *redis.Client {
	return c.client
}
