package testnet

import (
	"context"
	"fmt"
	"time"

	"github.com/celestiaorg/celestia-app/v3/test/e2e/testnet"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/knuu/pkg/instance"
)

const (
	appP2pPort = "26656"
)

type InstanceOptions struct {
	InstanceName string
	NodeType     node.Type
	Version      string
	Resources    testnet.Resources

	chainID     string
	genesisHash string
	executor    *instance.Instance
	consensus   *instance.Instance
}

func (o *InstanceOptions) Validate() error {
	if o.InstanceName == "" {
		return ErrEmptyInstanceName
	}
	if !o.NodeType.IsValid() {
		return ErrInvalidNodeType
	}
	if o.Version == "" {
		return ErrEmptyVersion
	}
	return nil
}

func (o *InstanceOptions) SetExecutor(executor *instance.Instance) {
	o.executor = executor
}

func (o *InstanceOptions) SetConsensus(consensus *instance.Instance) {
	o.consensus = consensus
}

func (o *InstanceOptions) Height(ctx context.Context) (int64, error) {
	if o.executor == nil {
		return 0, ErrExecutorNotSet
	}
	if o.consensus == nil {
		return 0, ErrConsensusNotSet
	}

	appHostName := o.consensus.Network().HostName()
	status, err := GetStatus(ctx, o.executor, appHostName)
	if err != nil {
		return 0, err
	}

	return status.LatestBlockHeight()
}

func (o *InstanceOptions) WaitForHeight(ctx context.Context, height int64) error {
	if o.executor == nil {
		return ErrExecutorNotSet
	}
	if o.consensus == nil {
		return ErrConsensusNotSet
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if ctx.Err() != nil {
				return ErrOperationCanceled.Wrap(ctx.Err())
			}
			return nil

		case <-ticker.C:
			blockHeight, err := o.Height(ctx)
			if err != nil {
				if _, ok := err.(*JSONRPCError); ok {
					// Retry if it's a temporary API error
					continue
				}
				return fmt.Errorf("error getting block height: %w", err)
			}

			if blockHeight >= height {
				return nil
			}
		}
	}
}

func (o *InstanceOptions) queryChainId(ctx context.Context) (string, error) {
	if o.executor == nil {
		return "", ErrExecutorNotSet
	}
	if o.consensus == nil {
		return "", ErrConsensusNotSet
	}

	appHostName := o.consensus.Network().HostName()
	status, err := GetStatus(ctx, o.executor, appHostName)
	if err != nil {
		return "", err
	}
	chainId, err := status.ChainID()
	if err != nil {
		return "", err
	}
	return chainId, nil
}
