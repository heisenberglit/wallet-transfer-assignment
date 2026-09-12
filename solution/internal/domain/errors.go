package domain

import "errors"

var (
	ErrWalletNotFound         = errors.New("wallet not found")
	ErrInsufficientFunds      = errors.New("insufficient funds")
	ErrInvalidAmount          = errors.New("amount must be positive")
	ErrSameWallet             = errors.New("fromWalletId and toWalletId must differ")
	ErrIdempotencyConflict    = errors.New("idempotency key already used")
	ErrInvalidStateTransition = errors.New("transfer is not in a state that allows this transition")
	ErrTransferInProgress     = errors.New("a transfer with this idempotency key is still in progress")
	ErrInconsistentLedger     = errors.New("ledger entries do not match the transfer")
)
