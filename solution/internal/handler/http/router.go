package http

import "net/http"

// NewRouter wires HTTP routes to handlers using Go 1.22+ stdlib pattern
// routing. Swap in a third-party router only if the assignment needs
// features ServeMux doesn't provide (e.g. middleware chaining helpers).
func NewRouter(transfers *TransferHandler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /transfers", transfers.Create)
	return mux
}
