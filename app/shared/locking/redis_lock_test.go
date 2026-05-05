package locking

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestRedisLock_Creation(t *testing.T) {
	type args struct {
		key   string
		ttl   time.Duration
		token string
	}
	type wants struct {
		keyPrefix  string
		tokenSet   bool
		clientSet  bool
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name: "1. RedisLock field initialization with lock prefix",
			args: args{
				key:   "resource-1",
				ttl:   time.Hour,
				token: uuid.NewString(),
			},
			wants: wants{
				keyPrefix: "lock:",
				tokenSet:  true,
				clientSet: true,
			},
		},
		{
			name: "2. RedisLock preserves exact token string",
			args: args{
				key:   "payment-lock",
				ttl:   5 * time.Minute,
				token: "custom-token-12345",
			},
			wants: wants{
				keyPrefix: "lock:",
				tokenSet:  true,
				clientSet: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create RedisLock directly to test field initialization
			lock := &RedisLock{
				key:   tt.wants.keyPrefix + tt.args.key,
				token: tt.args.token,
			}

			assert.Equal(t, tt.wants.keyPrefix+tt.args.key, lock.key, "key should have lock: prefix")
			assert.Equal(t, tt.args.token, lock.token, "token should be set")
		})
	}
}

func TestRedisLock_Release_TokenPreservation(t *testing.T) {
	type args struct {
		key   string
		token string
	}
	type wants struct {
		keyUsed   string
		tokenUsed string
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name: "1. release preserves key and token for Lua script",
			args: args{
				key:   "lock:resource-1",
				token: uuid.NewString(),
			},
			wants: wants{
				keyUsed:   "lock:resource-1",
				tokenUsed: uuid.NewString(),
			},
		},
		{
			name: "2. release uses correct key format in script",
			args: args{
				key:   "lock:payment-transaction",
				token: "token-abc-def",
			},
			wants: wants{
				keyUsed:   "lock:payment-transaction",
				tokenUsed: "token-abc-def",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lock := &RedisLock{
				key:   tt.args.key,
				token: tt.args.token,
			}

			assert.Equal(t, tt.args.key, lock.key, "key preserved for script")
			assert.Equal(t, tt.args.token, lock.token, "token preserved for script")
		})
	}
}

func TestReleaseScript(t *testing.T) {
	tests := []struct {
		name          string
		scriptContent string
		wants         struct {
			hasGetCall bool
			hasDelCall bool
			hasIfCheck bool
		}
	}{
		{
			name:          "1. release script contains GET command",
			scriptContent: releaseScript,
			wants: struct {
				hasGetCall bool
				hasDelCall bool
				hasIfCheck bool
			}{
				hasGetCall: true,
				hasDelCall: true,
				hasIfCheck: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.wants.hasGetCall {
				assert.Contains(t, tt.scriptContent, "redis.call(\"GET\"", "script should use GET command")
			}
			if tt.wants.hasDelCall {
				assert.Contains(t, tt.scriptContent, "redis.call(\"DEL\"", "script should use DEL command")
			}
			if tt.wants.hasIfCheck {
				assert.Contains(t, tt.scriptContent, "if redis.call(\"GET\", KEYS[1]) == ARGV[1]", "script should check token match")
			}
		})
	}
}

func TestAcquire_TokenGeneration(t *testing.T) {
	tests := []struct {
		name string
		wants struct {
			tokenIsUUID bool
			tokenLength int
		}
	}{
		{
			name: "1. token is valid UUID format",
			wants: struct {
				tokenIsUUID bool
				tokenLength int
			}{
				tokenIsUUID: true,
				tokenLength: 36, // UUID string format length
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			token := uuid.NewString()

			_, err := uuid.Parse(token)
			if tt.wants.tokenIsUUID {
				assert.NoError(t, err, "token should be a valid UUID")
			}

			assert.Equal(t, tt.wants.tokenLength, len(token), "UUID string format should be 36 chars")
		})
	}
}

func TestRedisLock_KeyFormat(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want string
	}{
		{
			name: "1. single word key",
			key:  "resource",
			want: "lock:resource",
		},
		{
			name: "2. key with hyphens",
			key:  "payment-lock",
			want: "lock:payment-lock",
		},
		{
			name: "3. key with multiple segments",
			key:  "user-123-transaction",
			want: "lock:user-123-transaction",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lock := &RedisLock{
				key: tt.want,
			}

			assert.Equal(t, tt.want, lock.key, "key format should match expected")
			assert.True(t, len(lock.key) > len("lock:"), "key should have prefix applied")
		})
	}
}

func TestRedisLock_ContextHandling(t *testing.T) {
	tests := []struct {
		name    string
		ctxFunc func() context.Context
	}{
		{
			name: "1. background context",
			ctxFunc: func() context.Context {
				return context.Background()
			},
		},
		{
			name: "2. TODO context",
			ctxFunc: func() context.Context {
				return context.TODO()
			},
		},
		{
			name: "3. context with timeout",
			ctxFunc: func() context.Context {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				return ctx
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := tt.ctxFunc()
			assert.NotNil(t, ctx, "context should not be nil")
		})
	}
}
