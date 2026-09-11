package domain

// TransferState models the lifecycle of a transfer.
//
// Allowed transitions (per ASSIGNMENT.md):
//
//	PENDING -> PROCESSED
//	PENDING -> FAILED
type TransferState string

const (
	TransferPending   TransferState = "PENDING"
	TransferProcessed TransferState = "PROCESSED"
	TransferFailed    TransferState = "FAILED"
)

// Transfer represents a single wallet-to-wallet transfer request.
//
// IdempotencyKey uniquely identifies a logical transfer request so that
// retried/duplicate API calls return the original result instead of
// creating a second transfer. TODO: enforce uniqueness at the DB level
// (see migrations/).
type Transfer struct {
	ID             string
	IdempotencyKey string
	FromWalletID   string
	ToWalletID     string
	Amount         int64
	State          TransferState
	CreatedAt      int64
	UpdatedAt      int64
}
