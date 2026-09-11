package domain

import "time"

// Wallet represents a source or destination account that transfers move funds between.
// Balance is a stored, updated column (not derived from LedgerEntry rows).
type Wallet struct {
	ID        string
	Balance   int64 // smallest currency unit (e.g. cents) to avoid floating point errors
	CreatedAt time.Time
	UpdatedAt time.Time
}
