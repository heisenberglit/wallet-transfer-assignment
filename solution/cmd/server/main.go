package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/heisenberglit/wallet-transfer-assignment/internal/db"
	httphandler "github.com/heisenberglit/wallet-transfer-assignment/internal/handler/http"
	"github.com/heisenberglit/wallet-transfer-assignment/internal/repository/postgres"
	"github.com/heisenberglit/wallet-transfer-assignment/internal/service"
)

func main() {
	ctx := context.Background()

	pool, err := db.Connect(ctx, db.LoadConfigFromEnv())
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer pool.Close()

	wallets := postgres.NewWalletRepository(pool)
	transfers := postgres.NewTransferRepository(pool)
	ledger := postgres.NewLedgerRepository(pool)
	uow := postgres.NewUnitOfWork(pool)

	transferService := service.NewTransferService(wallets, transfers, ledger, uow)
	transferHandler := httphandler.NewTransferHandler(transferService)
	router := httphandler.NewRouter(transferHandler)

	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
