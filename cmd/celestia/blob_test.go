package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/api/rpc"
	"github.com/celestiaorg/celestia-node/blob"
	blobmod "github.com/celestiaorg/celestia-node/nodebuilder/blob"
	blobcmd "github.com/celestiaorg/celestia-node/nodebuilder/blob/cmd"
)

type submittedBlobs struct {
	data [][]byte
}

func (s *submittedBlobs) Submit(_ context.Context, blobs []*blob.Blob, _ *blob.SubmitOptions) (uint64, error) {
	for _, b := range blobs {
		s.data = append(s.data, b.Data())
	}
	return 1, nil
}

func TestBlobSubmitData(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want string
	}{
		{name: "plain text", arg: "hello", want: "hello"},
		{name: "hex", arg: "0x676d", want: "gm"},
		{name: "plain text that looks like hex", arg: "cafe", want: "cafe"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			submitted := &submittedBlobs{}
			srv := rpc.NewServer("127.0.0.1", "0", true, rpc.CORSConfig{}, rpc.TLSConfig{}, rpc.RateLimitConfig{}, nil, nil)
			srv.RegisterService("blob", submitted, &blobmod.API{})
			require.NoError(t, srv.Start(context.Background()))
			t.Cleanup(func() { _ = srv.Stop(context.Background()) })
			t.Cleanup(func() {
				// don't leave the RPC flags pointing to the stopped server for other tests
				for _, name := range []string{"url", "token"} {
					f := blobcmd.Cmd.PersistentFlags().Lookup(name)
					_ = f.Value.Set(f.DefValue)
					f.Changed = false
				}
			})

			rootCmd.SetArgs([]string{
				"blob", "submit", "0x42690c204d39600fddd3", tt.arg,
				"--url", "http://" + srv.ListenAddr(), "--token", "test",
			})
			require.NoError(t, rootCmd.ExecuteContext(context.Background()))

			require.Len(t, submitted.data, 1)
			require.Equal(t, tt.want, string(submitted.data[0]))
		})
	}
}
