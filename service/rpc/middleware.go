package rpc

import (
	"errors"
	"net/http"
)

func (h *Handler) RegisterMiddlewares(rpc *Server) {
	h.registerHaltedMiddleware(rpc)
}

func (h *Handler) registerHaltedMiddleware(rpc *Server) {
	haltedTxMiddleware := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// check if state service was halted and deny the transation
		if h.state.IsHaltedSubmitTx() {
			writeError(w, http.StatusMethodNotAllowed, r.URL.Path, errors.New("not possible to submit data"))
			return
		} else {
			rpc.srvMux.ServeHTTP(w, r)
		}
	})

	rpc.srvMux.PathPrefix(submitTxEndpoint).Subrouter().MethodNotAllowedHandler = haltedTxMiddleware
	rpc.srvMux.PathPrefix(submitPFDEndpoint).Subrouter().MethodNotAllowedHandler = haltedTxMiddleware

}
