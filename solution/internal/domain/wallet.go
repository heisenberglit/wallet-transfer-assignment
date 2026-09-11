package domain

// Wallet represents a source or destination account that transfers move funds between.
//
// TODO: decide whether Balance is authoritative (stored/updated column) or
// derived from LedgerEntry rows. Document the decision in docs/design.md.
type Wallet struct {
	ID        string
	Balance   int64 // smallest currency unit (e.g. cents) to avoid floating point errors
	CreatedAt int64
	UpdatedAt int64
}
