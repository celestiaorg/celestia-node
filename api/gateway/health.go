package gateway

import "net/http"

const (
	healthEndpoint = "/status/health"
)

func (h *Handler) handleHealthRequest(w http.ResponseWriter, r *http.Request) {
	_, err := w.Write([]byte("ok"))
	if err != nil {
		log.Errorw("serving request", "endpoint", healthEndpoint, "err", err)
		writeError(w, http.StatusBadGateway, healthEndpoint, err)
		return
	}
}
