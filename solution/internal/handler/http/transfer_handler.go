package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/heisenberglit/wallet-transfer-assignment/internal/domain"
	"github.com/heisenberglit/wallet-transfer-assignment/internal/service"
)

// TransferCreator is the slice of TransferService that the HTTP layer
// depends on. Declaring it here (consumer side) rather than depending on
// the concrete *service.TransferService lets handler tests substitute a
// fake without spinning up a database.
type TransferCreator interface {
	CreateTransfer(ctx context.Context, in service.CreateTransferInput) (*domain.Transfer, error)
}

// TransferHandler exposes the transfer HTTP API. It only does request
// validation/decoding and transport mapping — no business logic.
type TransferHandler struct {
	transfers TransferCreator
}

func NewTransferHandler(transfers TransferCreator) *TransferHandler {
	return &TransferHandler{transfers: transfers}
}

// POST /transfers
func (h *TransferHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createTransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// TODO: request-level validation (required fields, amount > 0) belongs
	// here; business-rule validation (e.g. wallet existence) belongs in
	// the service layer.

	transfer, err := h.transfers.CreateTransfer(r.Context(), service.CreateTransferInput{
		IdempotencyKey: req.IdempotencyKey,
		FromWalletID:   req.FromWalletID,
		ToWalletID:     req.ToWalletID,
		Amount:         req.Amount,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, toTransferResponse(transfer))
}
