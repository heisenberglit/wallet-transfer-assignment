package http

import "net/http"

func NewRouter(transfers *TransferHandler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /transfers", transfers.Create)
	return Logging(mux)
}
