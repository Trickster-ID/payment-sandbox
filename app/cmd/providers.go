package main

import (
	"database/sql"

	"payment-sandbox/app/config"
	usersRepo "payment-sandbox/app/modules/users/repositories"
	"payment-sandbox/app/shared/journeylog"
)

func provideUserRepository(db *sql.DB) *usersRepo.UserRepository {
	return usersRepo.NewUserRepository(db)
}

func provideJourneyLogger(cfg config.Config) journeylog.IJourneyLogger {
	return journeylog.NewMongoJourneyLogger(cfg)
}
