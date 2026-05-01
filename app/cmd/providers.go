package main

import (
	"database/sql"
	"time"

	"payment-sandbox/app/shared/audit"
	"payment-sandbox/app/shared/idempotency"
	usersRepo "payment-sandbox/app/modules/users/repositories"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
)

func provideUserRepository(db *sql.DB) *usersRepo.UserRepository {
	return usersRepo.NewUserRepository(db)
}

func provideAuditLogger(db *mongo.Database) audit.IAuditLogger {
	return audit.NewLogger(db)
}

func provideIdempotencyMiddleware(db *sql.DB, rdb *redis.Client) *idempotency.Middleware {
	return &idempotency.Middleware{
		Store: &idempotency.Store{DB: db, TTL: 24 * time.Hour},
		Cache: &idempotency.Cache{Client: rdb, TTL: 24 * time.Hour},
	}
}
