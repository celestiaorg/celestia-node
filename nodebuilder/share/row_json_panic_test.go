package share

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/filecoin-project/go-jsonrpc"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share/shwap"
)

// TestRowUnmarshalJSONInvalidSide: shwap.Row.UnmarshalJSON panics on an unknown "side"
// value, so an RPC server (or anything between client and server) can crash a client that
// calls share.GetRow.
func TestRowUnmarshalJSONInvalidSide(t *testing.T) {
	t.Run("json.Unmarshal", func(t *testing.T) {
		var row shwap.Row
		require.NotPanics(t, func() {
			err := json.Unmarshal([]byte(`{"shares":[],"side":"UP"}`), &row)
			require.Error(t, err)
		})
	})

	t.Run("rpc client GetRow", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				ID json.RawMessage `json:"id"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":{"shares":[],"side":"UP"}}`, req.ID)
		}))
		defer srv.Close()

		var api API
		closer, err := jsonrpc.NewMergeClient(context.Background(), srv.URL, "share",
			[]interface{}{&api.Internal}, nil)
		require.NoError(t, err)
		defer closer()

		require.NotPanics(t, func() {
			_, err := api.GetRow(context.Background(), 1, 0)
			require.Error(t, err)
		})
	})
}
