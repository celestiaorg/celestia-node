package gateway

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	stateMock "github.com/celestiaorg/celestia-node/nodebuilder/state/mocks"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/state"
)

func TestHandleSubmitPFB(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := stateMock.NewMockModule(ctrl)
	handler := NewHandler(mock, nil, nil, nil)

	t.Run("partial response", func(t *testing.T) {
		txResponse := state.TxResponse{
			Height:    1,
			TxHash:    "hash",
			Codespace: "codespace",
			Code:      1,
		}
		// simulate core-app err, since it is not exported
		timedErr := errors.New("timed out waiting for tx to be included in a block")
		mock.EXPECT().SubmitPayForBlob(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&txResponse, timedErr)

		ns, err := share.NewBlobNamespaceV0([]byte("abc"))
		require.NoError(t, err)
		hexNs := hex.EncodeToString(ns[:])

		bs, err := json.Marshal(submitPFBRequest{
			NamespaceID: hexNs,
			Data:        "DEADBEEF",
		})
		require.NoError(t, err)
		httpreq := httptest.NewRequest("GET", "/", bytes.NewReader(bs))
		respRec := httptest.NewRecorder()
		handler.handleSubmitPFB(respRec, httpreq)

		var resp state.TxResponse
		err = json.NewDecoder(respRec.Body).Decode(&resp)
		require.NoError(t, err)

		require.Equal(t, http.StatusPartialContent, respRec.Code)
		require.Equal(t, resp, txResponse)
	})
}
