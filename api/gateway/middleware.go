package gateway

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gorilla/mux"

	"github.com/celestiaorg/celestia-node/nodebuilder/state"
)

const timeout = time.Minute

func (h *Handler) RegisterMiddleware(srv *Server) {
	srv.RegisterMiddleware(
		setContentType,
		checkPostDisabled(h.state),
		wrapRequestContext,
	)
}

func setContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

// checkPostDisabled ensures that context was canceled and prohibit POST requests.
func checkPostDisabled(state state.Module) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// check if state service was halted and deny the transaction
			if r.Method == http.MethodPost && state.IsStopped() {
				writeError(w, http.StatusMethodNotAllowed, r.URL.Path, errors.New("not possible to submit data"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
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
