package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"payment-sandbox/app/config"
	"payment-sandbox/app/modules/reconciliation/services"
	"payment-sandbox/app/shared/database"
)

func main() {
	ctx := context.Background()
	cfg := config.Load()

	db, err := database.New(cfg)
	if err != nil {
		fmt.Println("db:", err)
		os.Exit(1)
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
	os.Exit(exitCode)
}
