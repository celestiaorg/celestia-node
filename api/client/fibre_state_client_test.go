package client

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"testing"
	"time"

	p2pproto "github.com/cometbft/cometbft/proto/tendermint/p2p"
	tmservice "github.com/cosmos/cosmos-sdk/client/grpc/cmtservice"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/celestiaorg/celestia-app/v10/app"
	"github.com/celestiaorg/celestia-app/v10/app/encoding"
	valaddr "github.com/celestiaorg/celestia-app/v10/x/valaddr/types"

	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

type testNodeInfoServer struct {
	tmservice.UnimplementedServiceServer
}

func (*testNodeInfoServer) GetNodeInfo(
	context.Context, *tmservice.GetNodeInfoRequest,
) (*tmservice.GetNodeInfoResponse, error) {
	return &tmservice.GetNodeInfoResponse{DefaultNodeInfo: &p2pproto.DefaultNodeInfo{Network: "private"}}, nil
}

type testValaddrServer struct {
	valaddr.UnimplementedQueryServer
}

func (*testValaddrServer) AllBondedFibreProviders(
	context.Context, *valaddr.QueryAllBondedFibreProvidersRequest,
) (*valaddr.QueryAllBondedFibreProvidersResponse, error) {
	return &valaddr.QueryAllBondedFibreProvidersResponse{}, nil
}

func selfSignedTLSCert(t *testing.T) tls.Certificate {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "core"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

// TestInitTxClientOverTLSCoreEndpoint drives initTxClient against a consensus
// endpoint that only speaks TLS (CoreGRPCConfig.TLSEnabled), the normal setup
// for hosted gRPC providers. The tx client and CoreAccessor already dial this
// fine; this checks that the local Fibre state client (built inside
// initTxClient from the same conn) starts too, instead of failing because its
// default state client dials insecurely with no auth.
func TestInitTxClientOverTLSCoreEndpoint(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{selfSignedTLSCert(t)},
	})))
	tmservice.RegisterServiceServer(srv, &testNodeInfoServer{})
	valaddr.RegisterQueryServer(srv, &testValaddrServer{})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	// Same shape as grpcClient(CoreGRPCConfig{TLSEnabled: true}); the test cert
	// is self-signed so the client skips CA verification.
	tlsCfg := &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}
	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	encCfg := encoding.MakeConfig(app.ModuleEncodingRegisters...)
	kr := keyring.NewInMemory(encCfg.Codec)
	_, _, err = kr.NewMnemonic("test", keyring.English, "m/44'/118'/0'/0/0",
		keyring.DefaultBIP39Passphrase, hd.Secp256k1)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	c := &Client{}
	err = c.initTxClient(ctx, SubmitConfig{DefaultKeyName: "test", Network: p2p.Private}, conn, kr)
	require.NoError(t, err, "TLS core endpoint works for state/tx but the fibre state client dialed it in plaintext")
	_ = c.closer()
}
