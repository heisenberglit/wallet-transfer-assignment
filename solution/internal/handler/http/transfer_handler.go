package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/heisenberglit/wallet-transfer-assignment/internal/domain"
	"github.com/heisenberglit/wallet-transfer-assignment/internal/service"
)

// TransferCreator lets handler tests substitute a fake instead of *service.TransferService.
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

	if req.IdempotencyKey == "" || req.FromWalletID == "" || req.ToWalletID == "" {
		writeError(w, http.StatusBadRequest, "idempotencyKey, fromWalletId, and toWalletId are required")
		return
	}

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
