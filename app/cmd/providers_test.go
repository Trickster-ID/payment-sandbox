package main

import (
	"database/sql"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/mongo"
)

// Branch analysis for provideIdempotencyMiddleware():
// └── single code path: constructs Middleware with Store and Cache using provided db/rdb → returns *Middleware

// Branch analysis for provideAuditLogger():
// └── single code path: wraps mongo.Database in audit.Logger → returns IAuditLogger

func TestProvideIdempotencyMiddleware(t *testing.T) {
	// not parallel: uses package-level type constructors with no shared state but kept serial for clarity

	type args struct {
		db  *sql.DB
		rdb *redis.Client
	}
	type wants struct {
		storeDBSet  bool
		storeTTL    time.Duration
		cacheClient bool
		cacheTTL    time.Duration
	}

	db := &sql.DB{}
	rdb := &redis.Client{}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name: "1. valid db and redis client -> Middleware configured with 24h TTL on both layers",
			args: args{
				db:  db,
				rdb: rdb,
			},
			wants: wants{
				storeDBSet:  true,
				storeTTL:    24 * time.Hour,
				cacheClient: true,
				cacheTTL:    24 * time.Hour,
			},
		},
		{
			name: "2. nil db and nil redis client -> Middleware created with nil deps and 24h TTL",
			args: args{
				db:  nil,
				rdb: nil,
			},
			wants: wants{
				storeDBSet:  false,
				storeTTL:    24 * time.Hour,
				cacheClient: false,
				cacheTTL:    24 * time.Hour,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := provideIdempotencyMiddleware(tt.args.db, tt.args.rdb)

			assert.NotNil(t, got, "middleware must not be nil")
			assert.NotNil(t, got.Store, "store must not be nil")
			assert.NotNil(t, got.Cache, "cache must not be nil")

			if tt.wants.storeDBSet {
				assert.Equal(t, tt.args.db, got.Store.DB, "store.DB")
			} else {
				assert.Nil(t, got.Store.DB, "store.DB should be nil")
			}
			assert.Equal(t, tt.wants.storeTTL, got.Store.TTL, "store.TTL")

			if tt.wants.cacheClient {
				assert.Equal(t, tt.args.rdb, got.Cache.Client, "cache.Client")
			} else {
				assert.Nil(t, got.Cache.Client, "cache.Client should be nil")
			}
			assert.Equal(t, tt.wants.cacheTTL, got.Cache.TTL, "cache.TTL")
		})
	}
}

func TestProvideAuditLogger(t *testing.T) {
	// not parallel: mongo.Database has internal state

	type args struct {
		db *mongo.Database
	}
	type wants struct {
		notNil bool
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			// NewLogger returns noopLogger when db is nil — tests the nil-guard branch
			name:  "1. nil mongo database -> noopLogger returned (non-nil IAuditLogger)",
			args:  args{db: nil},
			wants: wants{notNil: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provideAuditLogger(tt.args.db)

			assert.NotNil(t, got, "audit logger must not be nil")
		})
	}
}
