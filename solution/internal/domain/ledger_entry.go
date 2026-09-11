package domain

import "time"

// LedgerEntryType identifies which side of a double-entry transaction a row represents.
type LedgerEntryType string

const (
	LedgerDebit  LedgerEntryType = "DEBIT"
	LedgerCredit LedgerEntryType = "CREDIT"
)

// LedgerEntry is one row of the double-entry ledger; every transfer
// produces exactly one DEBIT and one CREDIT with matching amounts.
type LedgerEntry struct {
	ID         string
	WalletID   string
	TransferID string
	Type       LedgerEntryType
	Amount     int64
	CreatedAt  time.Time
}
