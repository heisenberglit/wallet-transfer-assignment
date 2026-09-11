package domain

import "time"

// TransferState models the lifecycle: PENDING -> PROCESSED or PENDING -> FAILED.
type TransferState string

const (
	TransferPending   TransferState = "PENDING"
	TransferProcessed TransferState = "PROCESSED"
	TransferFailed    TransferState = "FAILED"
)

// Transfer represents a single wallet-to-wallet transfer request.
// IdempotencyKey is unique at the DB level (see migrations/).
type Transfer struct {
	ID             string
	IdempotencyKey string
	FromWalletID   string
	ToWalletID     string
	Amount         int64
	State          TransferState
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
