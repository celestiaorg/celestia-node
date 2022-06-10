package rpc

import (
	"net/http"
)

func (h *Handler) RegisterMiddleware(rpc *Server) {
	rpc.RegisterMiddleware(setContentType)
}

func setContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}
