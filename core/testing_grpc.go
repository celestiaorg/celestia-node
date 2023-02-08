package core

import (
	"net"
	"strings"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	srvconfig "github.com/cosmos/cosmos-sdk/server/config"
	"github.com/cosmos/cosmos-sdk/server/grpc/gogoreflection"
	reflection "github.com/cosmos/cosmos-sdk/server/grpc/reflection/v2alpha1"
	srvtypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/celestiaorg/celestia-app/testutil/testnode"
)

/*
StartGRPCServer is a copy of https://github.com/celestiaorg/celestia-app/blob/e5a679d11b464d583b616d4d686de9dd44bdab2e/testutil/testnode/rpc_client.go#L46
// It's copied as internal Cosmos SDK logic take 5 seconds to run: https://github.com/cosmos/cosmos-sdk/blob/6dfa0c98062d5d8b38d85ca1d2807937f47da4a3/server/grpc/server.go#L80
// FIXME once the fix for https://github.com/cosmos/cosmos-sdk/issues/14429 lands in our fork
*/
func StartGRPCServer(
	app srvtypes.Application,
	appCfg *srvconfig.Config,
	cctx testnode.Context,
) (testnode.Context, func() error, error) {
	emptycleanup := func() error { return nil }
	// Add the tx service in the gRPC router.
	app.RegisterTxService(cctx.Context)

	// Add the tendermint queries service in the gRPC router.
	app.RegisterTendermintService(cctx.Context)

	grpcSrv, err := startGRPCServer(cctx.Context, app, appCfg.GRPC)
	if err != nil {
		return testnode.Context{}, emptycleanup, err
	}

	nodeGRPCAddr := strings.Replace(appCfg.GRPC.Address, "0.0.0.0", "localhost", 1)
	conn, err := grpc.Dial(nodeGRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return testnode.Context{}, emptycleanup, err
	}

	cctx.Context = cctx.WithGRPCClient(conn)
	return cctx, func() error {
		grpcSrv.Stop()
		return nil
	}, nil
}

func startGRPCServer(
	clientCtx client.Context,
	app srvtypes.Application,
	cfg srvconfig.GRPCConfig,
) (*grpc.Server, error) {
	maxSendMsgSize := cfg.MaxSendMsgSize
	if maxSendMsgSize == 0 {
		maxSendMsgSize = srvconfig.DefaultGRPCMaxSendMsgSize
	}

	maxRecvMsgSize := cfg.MaxRecvMsgSize
	if maxRecvMsgSize == 0 {
		maxRecvMsgSize = srvconfig.DefaultGRPCMaxRecvMsgSize
	}

	grpcSrv := grpc.NewServer(
		grpc.ForceServerCodec(codec.NewProtoCodec(clientCtx.InterfaceRegistry).GRPCCodec()),
		grpc.MaxSendMsgSize(maxSendMsgSize),
		grpc.MaxRecvMsgSize(maxRecvMsgSize),
	)

	app.RegisterGRPCServer(grpcSrv)

	// Reflection allows consumers to build dynamic clients that can write to any
	// Cosmos SDK application without relying on application packages at compile
	// time.
	err := reflection.Register(grpcSrv, reflection.Config{
		SigningModes: func() map[string]int32 {
			modes := make(map[string]int32, len(clientCtx.TxConfig.SignModeHandler().Modes()))
			for _, m := range clientCtx.TxConfig.SignModeHandler().Modes() {
				modes[m.String()] = (int32)(m)
			}
			return modes
		}(),
		ChainID:           clientCtx.ChainID,
		SdkConfig:         sdk.GetConfig(),
		InterfaceRegistry: clientCtx.InterfaceRegistry,
	})
	if err != nil {
		return nil, err
	}

	// Reflection allows external clients to see what services and methods
	// the gRPC server exposes.
	gogoreflection.Register(grpcSrv)

	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return nil, err
	}

	go func() {
		err = grpcSrv.Serve(listener)
		if err != nil {
			log.Error("serving GRPC: ", err)
		}
	}()
	return grpcSrv, nil
}
