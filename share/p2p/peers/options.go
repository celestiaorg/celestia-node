package peers

import (
	"fmt"
	"time"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

type Parameters struct {
	// PoolValidationTimeout is the timeout used for validating incoming datahashes. Pools that have
	// been created for datahashes from shrexsub that do not see this hash from headersub after this
	// timeout will be garbage collected.
	PoolValidationTimeout time.Duration

	// PeerCooldown is the time a peer is put on cooldown after a ResultCooldownPeer.
	PeerCooldown time.Duration

	// GcInterval is the interval at which the manager will garbage collect unvalidated pools.
	GcInterval time.Duration

	// EnableBlackListing turns on blacklisting for misbehaved peers
	EnableBlackListing bool

	// NodeType is the type of node that is running
	// Used for protocol compatibility validation
	nodeType node.Type

	// NetworkID is the network ID of the node
	// Used for protocol compatibility validation
	networkID string
}

// Validate validates the values in Parameters
func (p *Parameters) Validate() error {
	if p.PoolValidationTimeout <= 0 {
		return fmt.Errorf("peer-manager: validation timeout must be positive")
	}

	if p.PeerCooldown <= 0 {
		return fmt.Errorf("peer-manager: peer cooldown must be positive")
	}

	if p.GcInterval <= 0 {
		return fmt.Errorf("peer-manager: garbage collection interval must be positive")
	}

	if p.nodeType.String() == "unknown" {
		return fmt.Errorf("peer-manager: node type must be set")
	}

	if p.networkID == "" {
		return fmt.Errorf("peer-manager: network ID must be set")
	}

	return nil
}

// DefaultParameters returns the default configuration values for the daser parameters
func DefaultParameters() Parameters {
	return Parameters{
		// PoolValidationTimeout's default value is based on the default daser sampling timeout of 1 minute.
		// If a received datahash has not tried to be sampled within these two minutes, the pool will be
		// removed.
		PoolValidationTimeout: 2 * time.Minute,
		// PeerCooldown's default value is based on initial network tests that showed a ~3.5 second
		// sync time for large blocks. This value gives our (discovery) peers enough time to sync
		// the new block before we ask them again.
		PeerCooldown: 3 * time.Second,
		GcInterval:   time.Second * 30,
		// blacklisting is off by default //TODO(@walldiss): enable blacklisting once all related issues
		// are resolved
		EnableBlackListing: false,
	}
}

// WithNodeType sets the node type for the peer manager parameters.
func (params *Parameters) WithNodeType(nodeType node.Type) {
	params.nodeType = nodeType
}

// WithNetworkID sets the network ID for the peer manager parameters.
func (params *Parameters) WithNetworkID(networkID string) {
	params.networkID = networkID
}

// WithMetrics turns on metric collection in peer manager.
func (m *Manager) WithMetrics() error {
	metrics, err := initMetrics(m)
	if err != nil {
		return fmt.Errorf("peer-manager: init metrics: %w", err)
	}
	m.metrics = metrics
	return nil
}
