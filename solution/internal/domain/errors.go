package domain

import "errors"

// Sentinel domain errors; handlers map these to HTTP status codes.
var (
	ErrWalletNotFound      = errors.New("wallet not found")
	ErrTransferNotFound    = errors.New("transfer not found")
	ErrInsufficientFunds   = errors.New("insufficient funds")
	ErrInvalidAmount       = errors.New("amount must be positive")
	ErrSameWallet          = errors.New("fromWalletId and toWalletId must differ")
	ErrIdempotencyConflict = errors.New("idempotency key reused with a different request payload")
)
