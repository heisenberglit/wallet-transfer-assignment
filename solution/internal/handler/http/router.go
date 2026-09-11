package http

import "net/http"

// NewRouter wires HTTP routes using Go 1.22+ stdlib pattern routing.
func NewRouter(transfers *TransferHandler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /transfers", transfers.Create)
	return Logging(mux)
}
