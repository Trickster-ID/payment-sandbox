package idempotency

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

type CachedResponse struct {
	RequestHash string `json:"h"`
	Code        int    `json:"c"`
	Body        []byte `json:"b"`
}

type Cache struct {
	Client *redis.Client
	TTL    time.Duration
}

func (c *Cache) Get(ctx context.Context, key string) (*CachedResponse, error) {
	raw, err := c.Client.Get(ctx, "idem:"+key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var resp CachedResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Cache) Set(ctx context.Context, key string, resp CachedResponse) error {
	b, _ := json.Marshal(resp)
	return c.Client.Set(ctx, "idem:"+key, b, c.TTL).Err()
}
