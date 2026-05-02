package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"payment-sandbox/app/config"
	"payment-sandbox/app/modules/reconciliation/services"
	"payment-sandbox/app/shared/database"
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
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		args = append(args, "main")
	}

	switch args[0] {
	case "main":
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
	case "reconcile":
		os.Exit(runReconcile())
	default:
		log.Fatalf("Arg [%s] is not valid.", args[0])
	}
}

func runReconcile() int {
	ctx := context.Background()
	cfg := config.Load()

	db, err := database.New(cfg)
	if err != nil {
		fmt.Println("db:", err)
		return 1
	}
	defer db.Close()

	runner := services.NewRunner(db)
	exitCode := 0

	if bad, err := runner.CheckTransactionBalance(ctx); err != nil {
		fmt.Println("tx balance check error:", err)
		exitCode = 2
	} else if len(bad) > 0 {
		fmt.Println("UNBALANCED TRANSACTIONS:")
		for _, b := range bad {
			fmt.Println(" ", b)
		}
		exitCode = 1
	}

	if bad, err := runner.CheckLedgerIntegrity(ctx); err != nil {
		fmt.Println("integrity check error:", err)
		exitCode = 2
	} else if len(bad) > 0 {
		fmt.Println("ACCOUNT INTEGRITY MISMATCH:")
		for _, b := range bad {
			fmt.Println(" ", b)
		}
		exitCode = 1
	}

	fmt.Printf("reconciliation finished at %s, exit=%d\n", time.Now().UTC().Format(time.RFC3339), exitCode)
	return exitCode
}
