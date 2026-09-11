package domain

import "errors"

// Sentinel domain errors. Handlers map these to HTTP status codes;
// services and repositories should return these (or wrap them) instead of
// leaking persistence-specific errors across layers.
var (
	ErrWalletNotFound      = errors.New("wallet not found")
	ErrInsufficientFunds   = errors.New("insufficient funds")
	ErrInvalidAmount       = errors.New("amount must be positive")
	ErrSameWallet          = errors.New("fromWalletId and toWalletId must differ")
	ErrIdempotencyConflict = errors.New("idempotency key reused with a different request payload")
)
