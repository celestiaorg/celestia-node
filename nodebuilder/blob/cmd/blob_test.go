package cmd

import (
	"context"
	"errors"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/filecoin-project/go-jsonrpc"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/blob"
	cmdnode "github.com/celestiaorg/celestia-node/cmd"
	blobmod "github.com/celestiaorg/celestia-node/nodebuilder/blob"
)

func TestSubmitArguments(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    []string
		wantErr string
	}{
		{
			name: "file without positional arguments",
			args: []string{"--input-file", "blobs.json"},
			want: []string{},
		},
		{
			name: "unprefixed namespace",
			args: []string{"0100", "hello"},
			want: []string{"0x0100", "hello"},
		},
		{
			name: "hex data",
			args: []string{"0x0100", "0x676d"},
			want: []string{"0x0100", "0x676d"},
		},
		{
			name:    "missing arguments",
			wantErr: "submit requires two arguments: namespace and blobData",
		},
		{
			name:    "missing data",
			args:    []string{"0100"},
			wantErr: "submit requires two arguments: namespace and blobData",
		},
		{
			name:    "extra arguments",
			args:    []string{"0100", "hello", "extra"},
			wantErr: "submit requires two arguments: namespace and blobData",
		},
		{
			name:    "file with positional arguments",
			args:    []string{"--input-file", "blobs.json", "0100", "ignored"},
			wantErr: "--input-file cannot be used with namespace or blobData arguments",
		},
		{
			name:    "invalid file extension",
			args:    []string{"--input-file", "blobs.txt"},
			wantErr: "invalid file extension, require json got .txt",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ran bool
			command := &cobra.Command{
				Use:     submitCmd.Use,
				Args:    submitCmd.Args,
				PreRunE: submitCmd.PreRunE,
				RunE: func(_ *cobra.Command, args []string) error {
					ran = true
					require.Equal(t, tt.want, args)
					return nil
				},
			}
			command.Flags().String(flagFileInput, "", "")
			command.SetOut(io.Discard)
			command.SetErr(io.Discard)
			command.SetArgs(tt.args)
			if tt.wantErr != "" {
				require.ErrorContains(t, command.Execute(), tt.wantErr)
				require.False(t, ran)
				return
			}
			require.NotPanics(t, func() {
				require.NoError(t, command.Execute())
			})
			require.True(t, ran)
		})
	}
}

func TestSubmitInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blobs.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"Blobs":[
		{"namespace":"0x0100","blobData":"hello"},
		{"namespace":"0x0200","blobData":"world"}
	]}`), 0o600))

	tests := []struct {
		name    string
		args    []string
		stdin   io.Reader
		want    []blobJSON
		wantErr string
	}{
		{
			name: "file",
			args: []string{"--input-file", path},
			want: []blobJSON{{Namespace: "0x0100", BlobData: "hello"}, {Namespace: "0x0200", BlobData: "world"}},
		},
		{
			name: "plain text",
			args: []string{"0100", "hello"},
			want: []blobJSON{{Namespace: "0x0100", BlobData: "hello"}},
		},
		{
			name: "hex data",
			args: []string{"0100", "0x676d"},
			want: []blobJSON{{Namespace: "0x0100", BlobData: "gm"}},
		},
		{
			name:  "raw stdin",
			args:  []string{"0100", "-"},
			stdin: strings.NewReader("0x676d\n\x00\xff"),
			want:  []blobJSON{{Namespace: "0x0100", BlobData: "0x676d\n\x00\xff"}},
		},
		{
			name:  "large stdin",
			args:  []string{"0100", "-"},
			stdin: strings.NewReader(strings.Repeat("payload\n", 8192)),
			want:  []blobJSON{{Namespace: "0x0100", BlobData: strings.Repeat("payload\n", 8192)}},
		},
		{
			name:    "stdin read error",
			args:    []string{"0100", "-"},
			stdin:   iotest.ErrReader(errors.New("stdin failed")),
			wantErr: "reading blob data from stdin: stdin failed",
		},
		{
			name:    "empty stdin",
			args:    []string{"0100", "-"},
			stdin:   strings.NewReader(""),
			wantErr: "error creating a blob",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			submitted := make(chan []*blob.Blob, 1)
			api := &blobmod.API{}
			api.Internal.Submit = func(_ context.Context, blobs []*blob.Blob, _ *blob.SubmitOptions) (uint64, error) {
				submitted <- blobs
				return 42, nil
			}
			rpcServer := jsonrpc.NewServer()
			rpcServer.Register("blob", api)
			server := httptest.NewServer(rpcServer)
			t.Cleanup(server.Close)

			command := &cobra.Command{
				Use:               submitCmd.Use,
				Args:              submitCmd.Args,
				PersistentPreRunE: cmdnode.InitClient,
				PreRunE:           submitCmd.PreRunE,
				RunE:              submitCmd.RunE,
			}
			command.Flags().String(flagFileInput, "", "")
			command.Flags().AddFlagSet(cmdnode.RPCFlags())
			command.SetArgs(append(tt.args, "--url", server.URL))
			command.SetIn(tt.stdin)
			command.SetOut(io.Discard)
			command.SetErr(io.Discard)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			err := command.ExecuteContext(ctx)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				require.Empty(t, submitted)
				return
			}
			require.NoError(t, err)

			select {
			case blobs := <-submitted:
				require.Len(t, blobs, len(tt.want))
				for i, want := range tt.want {
					namespace, err := cmdnode.ParseV0Namespace(want.Namespace)
					require.NoError(t, err)
					require.Equal(t, namespace, blobs[i].Namespace())
					require.Equal(t, []byte(want.BlobData), blobs[i].Data())
				}
			default:
				t.Fatal("blob.Submit was not called")
			}
		})
	}
}
