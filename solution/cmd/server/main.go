package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/heisenberglit/wallet-transfer-assignment/internal/db"
	httphandler "github.com/heisenberglit/wallet-transfer-assignment/internal/handler/http"
	"github.com/heisenberglit/wallet-transfer-assignment/internal/repository/postgres"
	"github.com/heisenberglit/wallet-transfer-assignment/internal/service"
)

const shutdownTimeout = 10 * time.Second

func main() {
	ctx := context.Background()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/wallet_transfer?sslmode=disable"
	}

	pool, err := db.Connect(ctx, dsn)
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

	srv := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	notifyCtx, stopNotify := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stopNotify()

	select {
	case err := <-serverErr:
		if err != nil {
			log.Fatalf("server error: %v", err)
		}
	case <-notifyCtx.Done():
		log.Println("shutdown signal received")

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("graceful shutdown failed: %v", err)
		}
	}
}
