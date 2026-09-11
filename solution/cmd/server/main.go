package main

import (
	"context"
	"errors"
	"log/slog"
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
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	ctx := context.Background()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/wallet_transfer?sslmode=disable"
	}

	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		slog.Error("connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	wallets := postgres.NewWalletRepository(pool)
	transfers := postgres.NewTransferRepository(pool)
	executor := postgres.NewTransferExecutor(pool)

	transferService := service.NewTransferService(wallets, transfers, executor)
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
		slog.Info("listening", "addr", addr)
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
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	case <-notifyCtx.Done():
		slog.Info("shutdown signal received")

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("graceful shutdown failed", "error", err)
		}
	}
}
