package domain

// LedgerEntryType identifies which side of a double-entry transaction a row represents.
type LedgerEntryType string

const (
	LedgerDebit  LedgerEntryType = "DEBIT"
	LedgerCredit LedgerEntryType = "CREDIT"
)

// LedgerEntry is one row of the double-entry ledger.
//
// Every transfer must produce exactly two entries (one DEBIT, one CREDIT)
// with matching amounts, so that the ledger always balances.
type LedgerEntry struct {
	ID         string
	WalletID   string
	TransferID string
	Type       LedgerEntryType
	Amount     int64
	CreatedAt  int64
}
