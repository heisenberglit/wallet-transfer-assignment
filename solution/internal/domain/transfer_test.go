package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/heisenberglit/wallet-transfer-assignment/internal/domain"
)

// TODO: replace with real behavioral tests once the transfer state
// machine and validation rules are implemented (see internal/service).
func TestTransferStates(t *testing.T) {
	assert.Equal(t, domain.TransferState("PENDING"), domain.TransferPending)
	assert.Equal(t, domain.TransferState("PROCESSED"), domain.TransferProcessed)
	assert.Equal(t, domain.TransferState("FAILED"), domain.TransferFailed)
}
