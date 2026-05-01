package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
)

// @title Payment Sandbox API
// @version 1.0
// @description Backend API for payment sandbox simulation.
// @BasePath /api/v1
// @schemes http
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	app, err := initApp()
	if err != nil {
		log.Fatal(err)
	}
	if err := app.Config.Validate(); err != nil {
		log.Fatal(err)
	}

	server := &http.Server{
		Addr:    ":" + app.Config.AppPort,
		Handler: app.Router,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app.Orchestrator.StartRecoveryLoop(ctx)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), app.Config.ShutdownTTL)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("graceful shutdown failed: %v", err)
		}
	}()

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
