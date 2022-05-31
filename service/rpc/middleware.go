package rpc

import (
	"errors"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/celestiaorg/celestia-node/service/state"
)

func (h *Handler) RegisterMiddlewares(rpc *Server) {
	rpc.RegisterMiddleware(setContentType)
	rpc.RegisterMiddleware(checkAllowance(h.state))
}

func setContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func checkAllowance(state *state.Service) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// check if state service was halted and deny the transaction
			if r.Method == http.MethodPost && state.BEFPReceived() {
				writeError(w, http.StatusMethodNotAllowed, r.URL.Path, errors.New("not possible to submit data"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
