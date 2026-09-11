package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/heisenberglit/wallet-transfer-assignment/internal/domain"
)

// Behavioral tests for the state machine live in internal/service.
func TestTransferStates(t *testing.T) {
	assert.Equal(t, domain.TransferState("PENDING"), domain.TransferPending)
	assert.Equal(t, domain.TransferState("PROCESSED"), domain.TransferProcessed)
	assert.Equal(t, domain.TransferState("FAILED"), domain.TransferFailed)
}
