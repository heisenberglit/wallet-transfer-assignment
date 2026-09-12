package domain

import "time"

// TransferState models the lifecycle: PENDING -> PROCESSED or PENDING -> FAILED.
type TransferState string

const (
	TransferPending   TransferState = "PENDING"
	TransferProcessed TransferState = "PROCESSED"
	TransferFailed    TransferState = "FAILED"
)

type Transfer struct {
	ID             string
	IdempotencyKey string
	FromWalletID   string
	ToWalletID     string
	Amount         int64 // minor units (e.g. cents), always positive
	State          TransferState
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
