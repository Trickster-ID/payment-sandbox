package database

// Branch map for NewRedis (Section 3.1 of the plan):
//
// NewRedis(cfg):
// ├── redis.ParseURL fails            -> return nil, error (wrapped with "parse REDIS_URL:")
// ├── client.Ping fails                -> return nil, error (wrapped with "redis ping:")
// └── all succeed                      -> return *redis.Client, nil

import (
	"testing"

	"payment-sandbox/app/config"

	"github.com/stretchr/testify/assert"
)

func TestNewRedis(t *testing.T) {
	type args struct {
		cfg config.Config
	}
	type wants struct {
		clientIsNil bool
		errMsg      string
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name: "1. invalid redis URL format -> parse error",
			args: args{cfg: config.Config{
				RedisURL: "not-a-valid-url",
			}},
			wants: wants{clientIsNil: true, errMsg: "parse REDIS_URL:"},
		},
		{
			name: "2. valid URL format, unreachable host -> ping error",
			args: args{cfg: config.Config{
				RedisURL: "redis://invalid-host:6379",
			}},
			wants: wants{clientIsNil: true, errMsg: "redis ping:"},
		},
		{
			name: "3. valid URL with empty string -> parse error",
			args: args{cfg: config.Config{
				RedisURL: "",
			}},
			wants: wants{clientIsNil: true, errMsg: "parse REDIS_URL:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, err := NewRedis(tt.args.cfg)

			assert.Nil(t, client, "client should be nil on error")
			assert.Error(t, err, "expected error")
			assert.Contains(t, err.Error(), tt.wants.errMsg, "error message contains expected text")
		})
	}
}
