package gateway

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWriteError(t *testing.T) {
	t.Run("writeError", func(t *testing.T) {
		// Create a mock HTTP response writer
		w := httptest.NewRecorder()

		testErr := errors.New("test error")

		writeError(w, http.StatusInternalServerError, "/api/endpoint", testErr)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		responseBody := w.Body.Bytes()
		assert.Equal(t, testErr.Error(), string(responseBody))
	})
}
