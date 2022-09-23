package rpc

import (
	"errors"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/celestiaorg/celestia-node/nodebuilder/state"
)

func (h *Handler) RegisterMiddleware(rpc *Server) {
	rpc.RegisterMiddleware(setContentType)
	rpc.RegisterMiddleware(checkPostDisabled(h.state))
}

func setContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

// checkPostDisabled ensures that context was canceled and prohibit POST requests.
func checkPostDisabled(state state.Service) mux.MiddlewareFunc {
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
