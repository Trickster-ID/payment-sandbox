package main

import (
	"database/sql"

	"payment-sandbox/app/config"
	authRepo "payment-sandbox/app/modules/auth/repositories"
	"payment-sandbox/app/shared/journeylog"
)

func provideAuthRepository(db *sql.DB) (*authRepo.SQLAuthRepository, error) {
	repo := authRepo.NewAuthRepository(db)
	if err := repo.EnsureAdminSeed(); err != nil {
		return nil, err
	}
	return repo, nil
}

func provideJourneyLogger(cfg config.Config) journeylog.JourneyLogger {
	return journeylog.NewMongoJourneyLogger(cfg)
}
