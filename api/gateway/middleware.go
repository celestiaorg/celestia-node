package gateway

import (
	"context"
	"net/http"
	"time"
)

const timeout = time.Minute

func (h *Handler) RegisterMiddleware(srv *Server) {
	srv.RegisterMiddleware(
		setContentType,
		wrapRequestContext,
		enableCors,
	)
}

func enableCors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		next.ServeHTTP(w, r)
	})
}

func setContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

// wrapRequestContext ensures we implement a deadline on serving requests
// via the gateway server-side to prevent context leaks.
func wrapRequestContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
