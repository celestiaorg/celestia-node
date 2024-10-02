package testnet

import (
	"context"
	"fmt"
	"strings"
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

func (o *InstanceOptions) ChainID(ctx context.Context) (string, error) {
	if o.chainID != "" {
		return o.chainID, nil
	}

	cid, err := o.queryChainId(ctx)
	if err != nil {
		return "", err
	}
	o.chainID = cid
	return cid, nil
}

func (o *InstanceOptions) GenesisHash(ctx context.Context) (string, error) {
	if o.genesisHash != "" {
		return o.genesisHash, nil
	}

	gh, err := o.queryGenesisHash(ctx)
	if err != nil {
		return "", err
	}
	o.genesisHash = gh
	return gh, nil
}

func (o *InstanceOptions) PersistentPeers(ctx context.Context, apps []*instance.Instance) (string, error) {
	if o.executor == nil {
		return "", ErrExecutorNotSet
	}

	var persistentPeers string
	for _, app := range apps {
		validatorIP, err := app.Network().GetIP(ctx)
		if err != nil {
			return "", ErrFailedToGetValidatorIP.Wrap(err)
		}

		status, err := GetStatus(ctx, o.executor, validatorIP)
		if err != nil {
			return "", err
		}
		id, err := status.NodeID()
		if err != nil {
			return "", err
		}
		persistentPeers += id + "@" + validatorIP + ":" + appP2pPort + ","
	}
	return strings.TrimSuffix(persistentPeers, ","), nil
}

func (o *InstanceOptions) Height(ctx context.Context) (int64, error) {
	if o.executor == nil {
		return 0, ErrExecutorNotSet
	}
	if o.consensus == nil {
		return 0, ErrConsensusNotSet
	}

	appIP, err := o.consensus.Network().GetIP(ctx)
	if err != nil {
		return 0, err
	}
	status, err := GetStatus(ctx, o.executor, appIP)
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

	appIP, err := o.consensus.Network().GetIP(ctx)
	if err != nil {
		return "", err
	}
	status, err := GetStatus(ctx, o.executor, appIP)
	if err != nil {
		return "", err
	}
	chainId, err := status.ChainID()
	if err != nil {
		return "", err
	}
	return chainId, nil
}

func (o *InstanceOptions) queryGenesisHash(ctx context.Context) (string, error) {
	if o.executor == nil {
		return "", ErrExecutorNotSet
	}
	if o.consensus == nil {
		return "", ErrConsensusNotSet
	}

	appIP, err := o.consensus.Network().GetIP(ctx)
	if err != nil {
		return "", ErrFailedToGetValidatorIP.Wrap(err)
	}

	block, err := o.executor.Execution().ExecuteCommand(ctx, "wget", "-q", "-O", "-", fmt.Sprintf("%s:26657/block?height=1", appIP))
	if err != nil {
		return "", ErrFailedToGetBlock.Wrap(err)
	}

	genesisHash, err := hashFromBlock(block)
	if err != nil {
		return "", ErrFailedToGetHashFromBlock.Wrap(err)
	}
	return genesisHash, nil
}
