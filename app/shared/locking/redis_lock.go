package locking

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type RedisLock struct {
	client *redis.Client
	key    string
	token  string
}

const releaseScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
else
    return 0
end
`

func Acquire(ctx context.Context, c *redis.Client, key string, ttl time.Duration) (*RedisLock, error) {
	token := uuid.NewString()
	ok, err := c.SetNX(ctx, "lock:"+key, token, ttl).Result()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("lock already held")
	}
	return &RedisLock{client: c, key: "lock:" + key, token: token}, nil
}

func (l *RedisLock) Release(ctx context.Context) error {
	_, err := l.client.Eval(ctx, releaseScript, []string{l.key}, l.token).Result()
	return err
}
