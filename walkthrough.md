# Celestia Node: A Complete Code Walkthrough

*2026-04-08T21:15:20Z by Showboat 0.6.1*
<!-- showboat-id: f78ee758-ec55-4a9b-a582-143e06a955db -->

Celestia Node is the data availability layer of the Celestia blockchain. It runs as a daemon process that participates in the peer-to-peer network to distribute, sample, and verify block data. The codebase supports three distinct node types -- Bridge, Light, and Full -- all built from the same modular architecture using uber/fx dependency injection.

This walkthrough traces the code linearly from the binary entry point through startup, configuration, dependency injection, and into each major subsystem: P2P networking, header synchronization, share exchange (SHWAP), data availability sampling, blob handling, state management, fraud detection, the RPC API, and pruning.

## 1. Entry Point: cmd/celestia/main.go

The binary starts here. The root Cobra command defines the CLI structure: `celestia [bridge | full | light] [subcommand]`. In `init()`, three top-level commands are registered -- one for each node type -- each decorated with the same set of subcommands (init, start, auth, etc.).

```bash
sed -n '26,65p' cmd/celestia/main.go
```

```output
func init() {
	bridgeCmd := cmdnode.NewBridge(WithSubcommands())
	lightCmd := cmdnode.NewLight(WithSubcommands())
	fullCmd := cmdnode.NewFull(WithSubcommands())
	rootCmd.AddCommand(
		bridgeCmd,
		lightCmd,
		fullCmd,
		docgenCmd,
		versionCmd,
	)
	rootCmd.SetHelpCommand(&cobra.Command{})
}

func main() {
	err := run()
	if err != nil {
		os.Exit(1)
	}
}

func run() error {
	return rootCmd.ExecuteContext(context.Background())
}

var rootCmd = &cobra.Command{
	Use: "celestia [  bridge  ||  full ||  light  ] [subcommand]",
	Short: `
	    ____      __          __  _
	  / ____/__  / /__  _____/ /_(_)___ _
	 / /   / _ \/ / _ \/ ___/ __/ / __  /
	/ /___/  __/ /  __(__  ) /_/ / /_/ /
	\____/\___/_/\___/____/\__/_/\__,_/
	`,
	Args: cobra.NoArgs,
	CompletionOptions: cobra.CompletionOptions{
		DisableDefaultCmd: false,
	},
}
```

Each node type command (`NewBridge`, `NewLight`, `NewFull`) is created with `WithSubcommands()`, which attaches the same set of lifecycle commands to each: `init`, `start`, `auth`, `reset-store`, `remove-config`, and `update-config`.

```bash
sed -n '13,24p' cmd/celestia/main.go
```

```output
func WithSubcommands() func(*cobra.Command, []*pflag.FlagSet) {
	return func(c *cobra.Command, flags []*pflag.FlagSet) {
		c.AddCommand(
			cmdnode.Init(flags...),
			cmdnode.Start(cmdnode.WithFlagSet(flags)),
			cmdnode.AuthCmd(flags...),
			cmdnode.ResetStore(flags...),
			cmdnode.RemoveConfigCmd(flags...),
			cmdnode.UpdateConfigCmd(flags...),
		)
	}
}
```

## 2. Node Types: Bridge, Light, Full

Before diving into startup, let's understand the three node types. They are defined as a simple `uint8` enum in `nodebuilder/node/type.go`:

```bash
sed -n '9,27p' nodebuilder/node/type.go
```

```output
// The zero value for Type is invalid.
type Type uint8

// StorePath is an alias used in order to pass the base path of the node store to nodebuilder
// modules.
type StorePath string

const (
	// Bridge is a Celestia Node that bridges the Celestia consensus network and data availability
	// network. It maintains a trusted channel/connection to a Celestia Core node via the core.Client
	// API.
	Bridge Type = iota + 1
	// Light is a stripped-down Celestia Node which aims to be lightweight while preserving the highest
	// possible security guarantees.
	Light
	// Full is a Celestia Node that stores blocks in their entirety.
	Full
)

```

- **Bridge** nodes connect directly to a Celestia Core (consensus) node, fetch blocks, construct ExtendedHeaders, and broadcast them over P2P. They are the bridge between the consensus and DA networks.
- **Light** nodes subscribe to headers from the P2P network and perform statistical Data Availability Sampling (DAS) -- they verify data is available without downloading entire blocks.
- **Full** nodes also subscribe to headers via P2P but download and store entire Extended Data Squares (EDS). They can serve share requests to other nodes.

This type flows through the entire codebase: almost every module's `ConstructModule` function takes a `node.Type` and switches on it to configure type-specific behavior.

## 3. The Start Command: cmd/start.go

The `start` command is where a node actually boots up. This is the most important command -- it parses flags, opens the store, constructs a keyring, builds the node via dependency injection, and then runs the lifecycle (start, wait for signal, stop).

```bash
sed -n '19,84p' cmd/start.go
```

```output
// Start constructs a CLI command to start Celestia Node daemon of any type with the given flags.
func Start(options ...func(*cobra.Command)) *cobra.Command {
	cmd := &cobra.Command{
		Use: "start",
		Short: `Starts Node daemon. First stopping signal gracefully stops the Node and second terminates it.
Options passed on start override configuration options only on start and are not persisted in config.`,
		Aliases:      []string{"run", "daemon"},
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			err = ParseAllFlags(cmd, NodeType(cmd.Context()), args)
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			// override config with all modifiers passed on start
			cfg := NodeConfig(ctx)

			storePath := StorePath(ctx)
			keysPath := filepath.Join(storePath, "keys")

			// construct ring
			// TODO @renaynay: Include option for setting custom `userInput` parameter with
			//  implementation of https://github.com/celestiaorg/celestia-node/issues/415.
			encConf := encoding.MakeConfig(app.ModuleEncodingRegisters...)
			ring, err := keyring.New(app.Name, cfg.State.DefaultBackendName, keysPath, os.Stdin, encConf.Codec)
			if err != nil {
				return err
			}

			store, err := nodebuilder.OpenStore(storePath, ring)
			if err != nil {
				return err
			}
			defer func() {
				err = errors.Join(err, store.Close())
			}()

			nd, err := nodebuilder.NewWithConfig(NodeType(ctx), Network(ctx), store, &cfg, NodeOptions(ctx)...)
			if err != nil {
				return err
			}

			ctx, cancel := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer cancel()
			err = nd.Start(ctx)
			if err != nil {
				return err
			}

			<-ctx.Done()
			cancel() // ensure we stop reading more signals for start context

			ctx, cancel = signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer cancel()
			return nd.Stop(ctx)
		},
	}
	// Apply each passed option to the command
	for _, option := range options {
		option(cmd)
	}
	return cmd
}
```

The startup sequence in `Start` is:

1. **Parse flags** -- CLI overrides take effect for this run only (not persisted to config)
2. **Open the keyring** -- Cosmos SDK keyring for signing transactions, stored under `<store>/keys/`
3. **Open the Store** -- Filesystem-backed store with file locking to prevent concurrent access
4. **Construct the Node** -- `nodebuilder.NewWithConfig()` uses uber/fx DI to wire up all modules
5. **Start the Node** -- Launches all components via fx lifecycle hooks
6. **Wait for signal** -- Blocks on SIGINT/SIGTERM
7. **Stop the Node** -- Graceful shutdown; a second signal forces termination

## 4. Configuration: nodebuilder/config.go

The `Config` struct aggregates configuration for every subsystem. Each field delegates to a subsystem-specific config. The config is persisted to disk as TOML and loaded on startup.

```bash
sed -n '26,60p' nodebuilder/config.go
```

```output
// It combines configuration units for all Node subsystems.
type Config struct {
	Node   node.Config
	Core   core.Config
	State  state.Config
	P2P    p2p.Config
	RPC    rpc.Config
	Share  share.Config
	Header header.Config
	DASer  das.Config `toml:",omitempty"`
}

// DefaultConfig provides a default Config for a given Node Type 'tp'.
// NOTE: Currently, configs are identical, but this will change.
func DefaultConfig(tp node.Type) *Config {
	commonConfig := &Config{
		Node:   node.DefaultConfig(tp),
		Core:   core.DefaultConfig(),
		State:  state.DefaultConfig(),
		P2P:    p2p.DefaultConfig(tp),
		RPC:    rpc.DefaultConfig(),
		Share:  share.DefaultConfig(tp),
		Header: header.DefaultConfig(tp),
	}

	switch tp {
	case node.Bridge:
		return commonConfig
	case node.Light, node.Full:
		commonConfig.DASer = das.DefaultConfig(tp)
		return commonConfig
	default:
		panic("node: invalid node type")
	}
}
```

Notice that `DASer` config is only populated for Light and Full nodes -- Bridge nodes connect directly to consensus and don't need to sample. The config is serialized to TOML via `BurntSushi/toml` and can be updated in place with `mergo.Merge` (which fills in new fields from defaults while preserving existing values).

## 5. The Store: nodebuilder/store.go

The Store provides persistent storage for node data. It's a filesystem-based implementation backed by BadgerDB (a high-performance key/value store), with file-locking to prevent multiple nodes from using the same store simultaneously.

```bash
sed -n '38,57p' nodebuilder/store.go
```

```output
// It provides access for the Node data stored in root directory e.g. '~/.celestia'.
type Store interface {
	// Path reports the FileSystem path of Store.
	Path() string

	// Keystore provides a Keystore to access keys.
	Keystore() (keystore.Keystore, error)

	// Datastore provides a Datastore - a KV store for arbitrary data to be stored on disk.
	Datastore() (datastore.Batching, error)

	// Config loads the stored Node config.
	Config() (*Config, error)

	// PutConfig alters the stored Node config.
	PutConfig(*Config) error

	// Close closes the Store freeing up acquired resources and locks.
	Close() error
}
```

The store directory defaults to `~/.celestia-<type>` for mainnet or `~/.celestia-<type>-<network>` for testnets. It can be overridden via the `CELESTIA_HOME` environment variable. Let's see how it resolves the path:

```bash
sed -n '190,211p' nodebuilder/store.go
```

```output
var DefaultNodeStorePath = func(tp nodemod.Type, network p2p.Network) (string, error) {
	home := os.Getenv("CELESTIA_HOME")
	if home != "" {
		return home, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	if network == p2p.Mainnet {
		return fmt.Sprintf("%s/.celestia-%s", home, strings.ToLower(tp.String())), nil
	}
	// only include network name in path for testnets and custom networks
	return fmt.Sprintf(
		"%s/.celestia-%s-%s",
		home,
		strings.ToLower(tp.String()),
		strings.ToLower(network.String()),
	), nil
}
```

The store is directory-locked via `gofrs/flock` -- if you try to start a second node on the same store, you get `ErrOpened`. Inside the store directory the layout is:

- `config.toml` -- the serialized Config
- `.lock` -- file lock preventing concurrent access
- `keys/` -- keystore for signing keys
- `data/` -- BadgerDB key/value store
- `blocks/` -- EDS files (for Bridge/Full nodes)
- `blocks/heights/` -- hardlinks from height numbers to EDS files

## 6. Node Construction: The DI System

This is the heart of celestia-node's architecture. The `Node` struct holds references to every subsystem -- P2P host, header service, share service, blob service, DAS, state, fraud detection, and the RPC server. It uses uber/fx's `fx.In` embedding to receive all dependencies via injection.

```bash
sed -n '47,83p' nodebuilder/node.go
```

```output
type Node struct {
	fx.In `ignore-unexported:"true"`

	Type          node.Type
	Network       p2p.Network
	Bootstrappers p2p.Bootstrappers
	Config        *Config
	AdminSigner   jwt.Signer

	// rpc components
	RPCServer *rpc.Server // not optional

	// block store
	EDSStore *store.Store `optional:"true"`

	// p2p components
	Host         host.Host
	ConnGater    *conngater.BasicConnectionGater
	Routing      routing.PeerRouting
	DataExchange exchange.SessionExchange
	BlockService blockservice.BlockService
	// p2p protocols
	PubSub *pubsub.PubSub
	// services
	ShareServ     share.Module  // not optional
	HeaderServ    header.Module // not optional
	StateServ     state.Module  // not optional
	FraudServ     fraud.Module  // not optional
	BlobServ      blob.Module   // not optional
	DASer         das.Module    // not optional
	AdminServ     node.Module   // not optional
	DAMod         da.Module     //nolint: staticcheck // not optional
	BlobstreamMod blobstream.Module

	// start and stop control ref internal fx.App lifecycle funcs to be called from Start and Stop
	start, stop lifecycleFunc
}
```

The `fx.In` embedding tells uber/fx to populate all exported fields from the DI container. The `start` and `stop` fields (unexported, hence `ignore-unexported`) are set directly from the fx.App's lifecycle methods after construction.

The `NewWithConfig` function wires everything together:

```bash
sed -n '95,100p' nodebuilder/node.go
```

```output
// NewWithConfig assembles a new Node with the given type 'tp' over Store 'store' and a custom
// config.
func NewWithConfig(tp node.Type, network p2p.Network, store Store, cfg *Config, options ...fx.Option) (*Node, error) {
	opts := append([]fx.Option{ConstructModule(tp, network, cfg, store)}, options...)
	return newNode(opts...)
}
```

And `newNode` creates the fx.App, populates the Node struct, and captures the lifecycle functions:

```bash
sed -n '171,196p' nodebuilder/node.go
```

```output
// newNode creates a new Node from given DI options.
// DI options allow initializing the Node with a customized set of components and services.
// NOTE: newNode is currently meant to be used privately to create various custom Node types e.g.
// Light, unless we decide to give package users the ability to create custom node types themselves.
func newNode(opts ...fx.Option) (*Node, error) {
	node := new(Node)
	app := fx.New(
		fx.WithLogger(func() fxevent.Logger {
			zl := &fxevent.ZapLogger{Logger: fxLog.Desugar()}
			zl.UseLogLevel(zapcore.DebugLevel)
			return zl
		}),
		fx.Populate(node),
		fx.Options(opts...),
	)
	if err := app.Err(); err != nil {
		return nil, err
	}

	node.start, node.stop = app.Start, app.Stop
	return node, nil
}

// lifecycleFunc defines a type for common lifecycle funcs.
type lifecycleFunc func(context.Context) error
```

The key line is `fx.Populate(node)` -- this tells fx to fill in every field of the Node struct from the DI container. And `node.start, node.stop = app.Start, app.Stop` captures the fx lifecycle so the Node can be started and stopped externally.

## 7. The Module System: nodebuilder/module.go

`ConstructModule` is where all the pieces come together. It builds the complete fx dependency graph by composing sub-modules for each subsystem. Every module gets the node type and its relevant config, so it can customize behavior accordingly.

```bash
cat nodebuilder/module.go
```

```output
package nodebuilder

import (
	"context"

	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/fxutil"
	"github.com/celestiaorg/celestia-node/nodebuilder/blob"
	"github.com/celestiaorg/celestia-node/nodebuilder/blobstream"
	"github.com/celestiaorg/celestia-node/nodebuilder/core"
	"github.com/celestiaorg/celestia-node/nodebuilder/da"
	"github.com/celestiaorg/celestia-node/nodebuilder/das"
	"github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	modhead "github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/nodebuilder/pruner"
	"github.com/celestiaorg/celestia-node/nodebuilder/rpc"
	"github.com/celestiaorg/celestia-node/nodebuilder/share"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"
)

func ConstructModule(tp node.Type, network p2p.Network, cfg *Config, store Store) fx.Option {
	log.Infow("Accessing keyring...")
	ks, err := store.Keystore()
	if err != nil {
		return fx.Error(err)
	}

	baseComponents := fx.Options(
		fx.Supply(tp),
		fx.Supply(network),
		fx.Supply(ks),
		fx.Provide(p2p.BootstrappersFor),
		fx.Provide(func(lc fx.Lifecycle) context.Context {
			return fxutil.WithLifecycle(context.Background(), lc)
		}),
		fx.Supply(cfg),
		fx.Supply(store.Config),
		fx.Provide(store.Datastore),
		fx.Provide(store.Keystore),
		core.ConstructModule(tp, &cfg.Core),
		fx.Supply(node.StorePath(store.Path())),
		// modules provided by the node
		p2p.ConstructModule(tp, &cfg.P2P),
		modhead.ConstructModule[*header.ExtendedHeader](tp, &cfg.Header),
		share.ConstructModule(tp, &cfg.Share),
		state.ConstructModule(tp, &cfg.State, &cfg.Core),
		das.ConstructModule(tp, &cfg.DASer),
		fraud.ConstructModule(tp),
		blob.ConstructModule(),
		da.ConstructModule(),
		node.ConstructModule(tp),
		pruner.ConstructModule(tp),
		rpc.ConstructModule(tp, &cfg.RPC),
		blobstream.ConstructModule(),
	)

	return fx.Module(
		"node",
		baseComponents,
	)
}
```

This is the assembly line. It:

1. **Supplies base values** into the DI container: the node type, network, keystore, config, datastore
2. **Provides a lifecycle-bound context** that gets canceled when the fx app stops
3. **Composes every subsystem module** -- each `ConstructModule` call returns an `fx.Option` that registers providers, suppliers, and lifecycle hooks

The order of module registration doesn't matter to fx -- it resolves the dependency graph at build time. Let's walk through each module.

## 8. P2P Module: nodebuilder/p2p/module.go

The P2P module sets up libp2p networking: the host, DHT routing, PubSub for gossip, connection management, and bandwidth monitoring.

```bash
sed -n '16,56p' nodebuilder/p2p/module.go
```

```output
	// sanitize config values before constructing module
	baseComponents := fx.Options(
		fx.Supply(cfg),
		fx.Provide(Key),
		fx.Provide(id),
		fx.Provide(peerStore),
		fx.Provide(connectionManager),
		fx.Provide(connectionGater),
		fx.Provide(newHost),
		fx.Provide(routedHost),
		fx.Provide(pubSub),
		fx.Provide(ipld.NewBlockservice),
		fx.Provide(peerRouting),
		fx.Provide(newDHT),
		fx.Provide(addrsFactory(cfg.AnnounceAddresses, cfg.NoAnnounceAddresses)),
		fx.Provide(metrics.NewBandwidthCounter),
		fx.Provide(newModule),
		fx.Invoke(Listen(cfg)),
		fx.Provide(resourceManager),
		fx.Provide(resourceManagerOpt(allowList)),
	)

	switch tp {
	case node.Full, node.Bridge:
		return fx.Module(
			"p2p",
			baseComponents,
			fx.Provide(infiniteResources),
			fx.Invoke(reachabilityCheck),
			fx.Invoke(connectToBootstrappers),
		)
	case node.Light:
		return fx.Module(
			"p2p",
			baseComponents,
			fx.Provide(autoscaleResources),
		)
	default:
		panic("invalid node type")
	}
}
```

The type-specific differences are significant:

- **Full/Bridge**: Get `infiniteResources` (no libp2p resource limits), actively check their reachability (NAT traversal), and proactively connect to bootstrapper nodes
- **Light**: Get `autoscaleResources` (dynamic resource limits based on available system resources) -- they are passive participants that don't need to be reachable

The provider chain builds up from key generation (`Key`) through peer identity (`id`), peer store, connection management, the libp2p host itself, and then the DHT and PubSub layers on top.

## 9. The Header System: header/header.go and nodebuilder/header/module.go

The `ExtendedHeader` is the core data type that flows through the entire system. It wraps a CometBFT consensus header with the additional information needed for data availability: the Commit, ValidatorSet, and crucially, the DataAvailabilityHeader (DAH).

```bash
sed -n '32,40p' header/header.go
```

```output
// ExtendedHeader represents a wrapped "raw" header that includes
// information necessary for Celestia Nodes to be notified of new
// block headers and perform Data Availability Sampling.
type ExtendedHeader struct {
	RawHeader    `json:"header"`
	Commit       *core.Commit               `json:"commit"`
	ValidatorSet *core.ValidatorSet         `json:"validator_set"`
	DAH          *da.DataAvailabilityHeader `json:"dah"`
}
```

The `DAH` (DataAvailabilityHeader) contains the row and column roots of the Extended Data Square -- these are the Merkle roots that allow anyone to verify individual shares without downloading the entire block.

The `Validate()` method performs thorough validation: basic header checks, version compatibility, commit validation, validator set consistency, DAH hash matching against the header's DataHash, and commit signature verification:

```bash
sed -n '110,163p' header/header.go
```

```output
func (eh *ExtendedHeader) Validate() error {
	err := eh.ValidateBasic()
	if err != nil {
		return fmt.Errorf("ValidateBasic error on RawHeader at height %d: %w", eh.Height(), err)
	}

	if eh.Version.App == 0 || eh.Version.App > appconsts.Version {
		return fmt.Errorf("header received at height %d has version %d, this node supports up "+
			"to version %d. Please upgrade to support new version. Note, 0 is not a valid version",
			eh.RawHeader.Height, eh.Version.App, appconsts.Version)
	}

	err = eh.Commit.ValidateBasic()
	if err != nil {
		return fmt.Errorf("ValidateBasic error on Commit at height %d: %w", eh.Height(), err)
	}

	err = eh.ValidatorSet.ValidateBasic()
	if err != nil {
		return fmt.Errorf("ValidateBasic error on ValidatorSet at height %d: %w", eh.Height(), err)
	}

	// make sure the validator set is consistent with the header
	if valSetHash := eh.ValidatorSet.Hash(); !bytes.Equal(eh.ValidatorsHash, valSetHash) {
		return fmt.Errorf("expected validator hash of header to match validator set hash (%X != %X) at height %d",
			eh.ValidatorsHash, valSetHash, eh.Height(),
		)
	}

	// ensure data root from raw header matches computed root
	if !bytes.Equal(eh.DAH.Hash(), eh.DataHash) {
		return fmt.Errorf("mismatch between data hash commitment from core header and computed data root "+
			"at height %d: data hash: %X, computed root: %X", eh.Height(), eh.DataHash, eh.DAH.Hash())
	}

	// Make sure the header is consistent with the commit.
	if eh.Commit.Height != eh.RawHeader.Height {
		return fmt.Errorf("header and commit height mismatch: %d vs %d", eh.RawHeader.Height, eh.Commit.Height)
	}
	if hhash, chash := eh.RawHeader.Hash(), eh.Commit.BlockID.Hash; !bytes.Equal(hhash, chash) {
		return fmt.Errorf("commit signs block %X, header is block %X", chash, hhash)
	}

	err = eh.ValidatorSet.VerifyCommitLight(eh.ChainID(), eh.Commit.BlockID, int64(eh.Height()), eh.Commit)
	if err != nil {
		return fmt.Errorf("VerifyCommitLight error at height %d: %w", eh.Height(), err)
	}

	err = eh.DAH.ValidateBasic()
	if err != nil {
		return fmt.Errorf("ValidateBasic error on DAH at height %d: %w", eh.RawHeader.Height, err)
	}
	return nil
}
```

Now let's see the header module construction, which sets up the syncer, P2P subscriber, exchange server, and header store:

```bash
sed -n '26,124p' nodebuilder/header/module.go
```

```output
func ConstructModule[H libhead.Header[H]](tp node.Type, cfg *Config) fx.Option {
	// sanitize config values before constructing module
	cfgErr := cfg.Validate(tp)

	baseComponents := fx.Options(
		fx.Supply(*cfg),
		fx.Error(cfgErr),
		fx.Provide(newHeaderService),
		fx.Provide(newStore[H]),
		fx.Provide(func(subscriber *p2p.Subscriber[H]) libhead.Subscriber[H] {
			return subscriber
		}),
		fx.Provide(newSyncer[H]),
		fx.Provide(fx.Annotate(
			newFraudedSyncer[H],
			fx.OnStart(func(
				ctx context.Context,
				breaker *modfraud.ServiceBreaker[*sync.Syncer[H], H],
			) error {
				// TODO(@Wondertan): This fix flakes in e2e tests
				//  This is coming from the store asynchronity.
				//  Previously, we would request genesis during initialization
				//  but now we request it during Syncer start and given to the Store.
				//  However, the Store doesn't makes it immediately available causing flakes
				//  The proper fix will be in a follow up release after pruning.
				defer time.Sleep(time.Millisecond * 100)
				return breaker.Start(ctx)
			}),
			fx.OnStop(func(
				ctx context.Context,
				breaker *modfraud.ServiceBreaker[*sync.Syncer[H], H],
			) error {
				return breaker.Stop(ctx)
			}),
		)),
		fx.Provide(fx.Annotate(
			func(ps *pubsub.PubSub, network modp2p.Network) (*p2p.Subscriber[H], error) {
				opts := []p2p.SubscriberOption{p2p.WithSubscriberNetworkID(network.String())}
				if MetricsEnabled {
					opts = append(opts, p2p.WithSubscriberMetrics())
				}
				return p2p.NewSubscriber[H](ps, header.MsgID, opts...)
			},
			fx.OnStart(func(ctx context.Context, sub *p2p.Subscriber[H]) error {
				return sub.Start(ctx)
			}),
			fx.OnStop(func(ctx context.Context, sub *p2p.Subscriber[H]) error {
				return sub.Stop(ctx)
			}),
		)),
		fx.Provide(fx.Annotate(
			func(
				cfg Config,
				host host.Host,
				store libhead.Store[H],
				network modp2p.Network,
			) (*p2p.ExchangeServer[H], error) {
				opts := []p2p.Option[p2p.ServerParameters]{
					p2p.WithParams(cfg.Server),
					p2p.WithNetworkID[p2p.ServerParameters](network.String()),
				}
				if MetricsEnabled {
					opts = append(opts, p2p.WithMetrics[p2p.ServerParameters]())
				}

				return p2p.NewExchangeServer[H](host, store, opts...)
			},
			fx.OnStart(func(ctx context.Context, server *p2p.ExchangeServer[H]) error {
				return server.Start(ctx)
			}),
			fx.OnStop(func(ctx context.Context, server *p2p.ExchangeServer[H]) error {
				return server.Stop(ctx)
			}),
		)),
	)

	switch tp {
	case node.Light, node.Full:
		return fx.Module(
			"header",
			baseComponents,
			fx.Provide(newP2PExchange[H]),
			fx.Provide(func(ctx context.Context, ds datastore.Batching) (p2p.PeerIDStore, error) {
				return pidstore.NewPeerIDStore(ctx, ds)
			}),
		)
	case node.Bridge:
		return fx.Module(
			"header",
			baseComponents,
			fx.Provide(func(subscriber *p2p.Subscriber[H]) libhead.Broadcaster[H] {
				return subscriber
			}),
			fx.Supply(header.MakeExtendedHeader),
		)
	default:
		panic("invalid node type")
	}
}
```

This is generics-heavy -- `ConstructModule[H libhead.Header[H]]` is instantiated with `*header.ExtendedHeader`. The key components:

- **Header Store** -- persists validated headers
- **Syncer** -- wrapped in a `ServiceBreaker` (fraud-protected) that stops syncing if a fraud proof is detected
- **P2P Subscriber** -- listens for new headers gossiped over PubSub
- **Exchange Server** -- serves header requests to other peers

The type-specific differences:
- **Light/Full**: Use a P2P Exchange *client* to request headers from peers, with a PeerIDStore to track trusted header sources
- **Bridge**: Uses the P2P Subscriber as a Broadcaster (it publishes headers *to* the network rather than consuming them), and supplies the `MakeExtendedHeader` constructor function (used by the Core module to build headers from consensus blocks)

## 10. Core Module: nodebuilder/core/module.go

The Core module manages the relationship with the Celestia Core consensus node. This module is fundamentally different between Bridge and non-Bridge nodes.

```bash
sed -n '21,84p' nodebuilder/core/module.go
```

```output
func ConstructModule(tp node.Type, cfg *Config, options ...fx.Option) fx.Option {
	// sanitize config values before constructing module
	cfgErr := cfg.Validate()

	baseComponents := fx.Options(
		fx.Supply(*cfg),
		fx.Supply(cfg.EndpointConfig),
		fx.Error(cfgErr),
		fx.Provide(grpcClient),
		fx.Provide(additionalCoreEndpointGrpcClients),
		fx.Options(options...),
	)

	switch tp {
	case node.Light, node.Full:
		return fx.Module("core", baseComponents)
	case node.Bridge:
		return fx.Module("core",
			baseComponents,
			fx.Provide(core.NewBlockFetcher),
			fxutil.ProvideAs(
				func(
					fetcher *core.BlockFetcher,
					store *store.Store,
					construct header.ConstructFn,
					opts []core.Option,
				) (*core.Exchange, error) {
					if MetricsEnabled {
						opts = append(opts, core.WithMetrics())
					}

					return core.NewExchange(fetcher, store, construct, opts...)
				},
				new(libhead.Exchange[*header.ExtendedHeader])),
			fx.Invoke(fx.Annotate(
				func(
					bcast libhead.Broadcaster[*header.ExtendedHeader],
					fetcher *core.BlockFetcher,
					pubsub *shrexsub.PubSub,
					construct header.ConstructFn,
					store *store.Store,
					chainID p2p.Network,
					opts []core.Option,
				) (*core.Listener, error) {
					opts = append(opts, core.WithChainID(chainID))

					if MetricsEnabled {
						opts = append(opts, core.WithMetrics())
					}

					return core.NewListener(bcast, fetcher, pubsub.Broadcast, construct, store, p2p.BlockTime, opts...)
				},
				fx.OnStart(func(ctx context.Context, listener *core.Listener) error {
					return listener.Start(ctx)
				}),
				fx.OnStop(func(ctx context.Context, listener *core.Listener) error {
					return listener.Stop(ctx)
				}),
			)),
		)
	default:
		panic("invalid node type")
	}
}
```

For **Light/Full** nodes, the Core module just provides a gRPC client (used by the state module for transaction submission). The real action is in **Bridge** nodes, which get three additional components:

1. **BlockFetcher** -- pulls blocks from the Core node via gRPC
2. **Exchange** -- wraps the fetcher to satisfy the `libhead.Exchange` interface, constructing `ExtendedHeader`s from raw blocks using the `ConstructFn` and storing the resulting EDS in the Store
3. **Listener** -- subscribes to new blocks from Core, constructs ExtendedHeaders, stores the EDS, and broadcasts headers over P2P via the `Broadcaster` and share data hashes via `shrexsub.PubSub`

This is the critical Bridge data flow: **Core consensus -> BlockFetcher -> Listener -> (store EDS + broadcast header + broadcast shrexsub notification) -> P2P network**

## 11. Share Module & Data Availability: nodebuilder/share/module.go

The Share module is one of the most complex modules. It handles everything related to fetching, serving, and validating share data -- the fundamental data units of Celestia's erasure-coded blocks.

```bash
sed -n '27,61p' nodebuilder/share/module.go
```

```output
func ConstructModule(tp node.Type, cfg *Config, options ...fx.Option) fx.Option {
	// sanitize config values before constructing module
	err := cfg.Validate(tp)
	if err != nil {
		return fx.Error(fmt.Errorf("nodebuilder/share: validate config: %w", err))
	}

	baseComponents := fx.Options(
		fx.Supply(*cfg),
		fx.Options(options...),
		fx.Provide(newShareModule),
		availabilityComponents(tp, cfg),
		shrexComponents(tp, cfg),
		bitswapComponents(tp, cfg),
		peerManagementComponents(tp, cfg),
	)

	switch tp {
	case node.Bridge, node.Full:
		return fx.Module(
			"share",
			baseComponents,
			edsStoreComponents(cfg),
			fx.Provide(bridgeAndFullGetter),
		)
	case node.Light:
		return fx.Module(
			"share",
			baseComponents,
			fx.Provide(lightGetter),
		)
	default:
		panic("invalid node type")
	}
}
```

The module is composed of several sub-component groups:

- **availabilityComponents** -- the data availability validator, which differs by node type
- **shrexComponents** -- ShrEx (Share Exchange) protocol: a custom peer-to-peer share exchange built on libp2p streams
- **bitswapComponents** -- IPFS Bitswap-based block exchange, an alternative retrieval mechanism
- **peerManagementComponents** -- manages pools of peers for share retrieval
- **edsStoreComponents** -- (Bridge/Full only) the EDS file store for persisting complete blocks

**Bridge/Full** nodes get the EDS store and a `bridgeAndFullGetter` that can fetch shares from the local store. **Light** nodes get a `lightGetter` that must fetch shares from the network.

The availability components are particularly important -- they determine how each node type validates data availability:

```bash
sed -n '210,246p' nodebuilder/share/module.go
```

```output
func availabilityComponents(tp node.Type, cfg *Config) fx.Option {
	switch tp {
	case node.Light:
		return fx.Options(
			fx.Provide(fx.Annotate(
				func(getter shwap.Getter, ds datastore.Batching, bs blockstore.Blockstore) *light.ShareAvailability {
					return light.NewShareAvailability(
						getter,
						ds,
						bs,
						light.WithSampleAmount(cfg.LightAvailability.SampleAmount),
					)
				},
				fx.As(fx.Self()),
				fx.As(new(share.Availability)),
				fx.OnStop(func(ctx context.Context, la *light.ShareAvailability) error {
					return la.Close(ctx)
				}),
			)),
		)
	case node.Bridge, node.Full:
		return fx.Options(
			fx.Provide(func(
				s *store.Store,
				getter shwap.Getter,
				opts []full.Option,
			) *full.ShareAvailability {
				return full.NewShareAvailability(s, getter, opts...)
			}),
			fx.Provide(func(avail *full.ShareAvailability) share.Availability {
				return avail
			}),
		)
	default:
		panic("invalid node type")
	}
}
```

- **Light availability** (`light.ShareAvailability`): Performs *statistical sampling* -- it randomly samples a configurable number of shares from the block and checks they are retrievable. If enough random samples succeed, it concludes (with high probability) that the full data is available.
- **Full availability** (`full.ShareAvailability`): Downloads the *complete* data square and verifies it. Uses the EDS store and getter to ensure all data is actually retrievable.

## 12. The EDS Store: store/store.go

Bridge and Full nodes persist Extended Data Squares on disk. The `Store` manages these as flat files with two file formats: ODS (Original Data Square -- just the first quadrant) and Q4 (the fourth quadrant of the erasure-coded extension).

```bash
sed -n '38,87p' store/store.go
```

```output
// used to cache blocks that are accessed by sample requests. Store is thread-safe.
type Store struct {
	// basepath is the root directory of the store
	basepath string
	// cache is used to cache recent blocks and blocks that are accessed frequently
	cache cache.Cache
	// stripedLocks is used to synchronize parallel operations
	stripLock *striplock
	metrics   *metrics
}

// NewStore creates a new EDS Store under the given basepath and datastore.
func NewStore(params *Parameters, basePath string) (*Store, error) {
	err := params.Validate()
	if err != nil {
		return nil, err
	}

	// ensure the blocks dir exists
	blocksDir := filepath.Join(basePath, blocksPath)
	if err := mkdir(blocksDir); err != nil {
		return nil, fmt.Errorf("ensuring blocks directory: %w", err)
	}

	// ensure the heights dir exists
	heightsDir := filepath.Join(basePath, heightsPath)
	if err := mkdir(heightsDir); err != nil {
		return nil, fmt.Errorf("ensuring heights directory: %w", err)
	}

	var recentCache cache.Cache = cache.NoopCache{}
	if params.RecentBlocksCacheSize > 0 {
		recentCache, err = cache.NewAccessorCache("recent", params.RecentBlocksCacheSize)
		if err != nil {
			return nil, fmt.Errorf("failed to create recent eds cache: %w", err)
		}
	}

	store := &Store{
		basepath:  basePath,
		cache:     recentCache,
		stripLock: newStripLock(1024),
	}

	if err := store.populateEmptyFile(); err != nil {
		return nil, fmt.Errorf("ensuring empty EDS: %w", err)
	}

	return store, nil
}
```

Key design choices in the EDS Store:

- **Files are addressed by data hash** -- the ODS file path is `blocks/<datahash>.ods`
- **Heights are mapped via hard links** -- `blocks/heights/<height>.ods` is a hard link to the hash-named file. This means the same EDS can be looked up by either hash or height efficiently
- **Empty blocks use symlinks** -- since many blocks may be empty, they all symlink to a single canonical empty EDS file (avoids hitting filesystem hard link limits)
- **Striped locks** -- 1024 striped locks allow high-concurrency parallel reads/writes without a single global lock
- **Cache layer** -- a recent blocks cache avoids repeated file I/O for hot data
- **Corruption recovery** -- on `Put`, if a file already exists but has the wrong size, it's automatically deleted and recreated

## 13. Data Availability Sampling: das/daser.go

The DASer is the engine that continuously validates data availability. It only runs on Light and Full nodes (Bridge nodes have the data directly from Core). It subscribes to new headers and samples shares to verify they are available on the network.

```bash
sed -n '23,78p' das/daser.go
```

```output
// DASer continuously validates availability of data committed to headers.
type DASer struct {
	params Parameters

	da     share.Availability
	bcast  fraud.Broadcaster[*header.ExtendedHeader]
	hsub   libhead.Subscriber[*header.ExtendedHeader] // listens for new headers in the network
	getter libhead.Store[*header.ExtendedHeader]      // retrieves past headers

	sampler    *samplingCoordinator
	store      checkpointStore
	subscriber subscriber

	cancel         context.CancelFunc
	subscriberDone chan struct{}
	running        atomic.Bool
}

type (
	listenFn func(context.Context, *header.ExtendedHeader)
	sampleFn func(context.Context, *header.ExtendedHeader) error
)

// NewDASer creates a new DASer.
func NewDASer(
	da share.Availability,
	hsub libhead.Subscriber[*header.ExtendedHeader],
	getter libhead.Store[*header.ExtendedHeader],
	dstore datastore.Datastore,
	bcast fraud.Broadcaster[*header.ExtendedHeader],
	shrexBroadcast shrexsub.BroadcastFn,
	options ...Option,
) (*DASer, error) {
	d := &DASer{
		params:         DefaultParameters(),
		da:             da,
		bcast:          bcast,
		hsub:           hsub,
		getter:         getter,
		store:          newCheckpointStore(dstore),
		subscriber:     newSubscriber(),
		subscriberDone: make(chan struct{}),
	}

	for _, applyOpt := range options {
		applyOpt(d)
	}

	err := d.params.Validate()
	if err != nil {
		return nil, err
	}

	d.sampler = newSamplingCoordinator(d.params, getter, d.sample, shrexBroadcast)
	return d, nil
}
```

The DASer's Start method loads a checkpoint (so it can resume from where it left off after restart), subscribes to new headers, and launches three goroutines:

```bash
sed -n '81,104p' das/daser.go
```

```output
func (d *DASer) Start(ctx context.Context) error {
	if !d.running.CompareAndSwap(false, true) {
		return errors.New("da: DASer already started")
	}

	cp, err := d.checkpoint(ctx)
	if err != nil {
		return err
	}

	sub, err := d.hsub.Subscribe()
	if err != nil {
		return err
	}

	runCtx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel

	go d.sampler.run(runCtx, cp)
	go d.subscriber.run(runCtx, sub, d.sampler.listen)
	go d.store.runBackgroundStore(runCtx, d.params.BackgroundStoreInterval, d.sampler.getCheckpoint)

	return nil
}
```

Three concurrent goroutines work together:

1. **`sampler.run`** -- the sampling coordinator that manages worker goroutines to sample header ranges
2. **`subscriber.run`** -- listens for new headers from P2P and feeds them to the sampler
3. **`store.runBackgroundStore`** -- periodically persists the sampling checkpoint to disk

When the DASer samples a header, it calls `d.da.SharesAvailable()`. If that returns a `*byzantine.ErrByzantine`, it means the node detected invalid erasure coding -- a fraud proof is created and broadcast to the network:

```bash
sed -n '201,215p' das/daser.go
```

```output
func (d *DASer) sample(ctx context.Context, h *header.ExtendedHeader) error {
	err := d.da.SharesAvailable(ctx, h)
	if err != nil {
		var byzantineErr *byzantine.ErrByzantine
		if errors.As(err, &byzantineErr) {
			log.Warn("Propagating proof...")
			sendErr := d.bcast.Broadcast(ctx, byzantine.CreateBadEncodingProof(h.Hash(), h.Height(), byzantineErr))
			if sendErr != nil {
				log.Errorw("fraud proof propagating failed", "err", sendErr)
			}
		}
		return err
	}
	return nil
}
```

The DAS module is wrapped in a fraud `ServiceBreaker`. Let's see how that works in the DAS module construction:

```bash
cat nodebuilder/das/module.go
```

```output
package das

import (
	"context"

	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/das"
	"github.com/celestiaorg/celestia-node/header"
	modfraud "github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

func ConstructModule(tp node.Type, cfg *Config) fx.Option {
	// If DASer is disabled, provide the stub implementation for any node type
	// Also provide the stub implementation for bridge nodes as they do not need DASer
	if !cfg.Enabled || tp == node.Bridge {
		return fx.Module(
			"das",
			fx.Provide(newDaserStub),
		)
	}

	baseComponents := fx.Options(
		fx.Supply(*cfg),
		fx.Error(cfg.Validate()),
		fx.Provide(
			func(c Config) []das.Option {
				return []das.Option{
					das.WithSamplingRange(c.SamplingRange),
					das.WithConcurrencyLimit(c.ConcurrencyLimit),
					das.WithBackgroundStoreInterval(c.BackgroundStoreInterval),
					das.WithSampleTimeout(c.SampleTimeout),
				}
			},
		),
	)

	return fx.Module(
		"das",
		baseComponents,
		fx.Provide(fx.Annotate(
			newDASer,
			fx.OnStart(func(ctx context.Context, breaker *modfraud.ServiceBreaker[*das.DASer, *header.ExtendedHeader]) error {
				return breaker.Start(ctx)
			}),
			fx.OnStop(func(ctx context.Context, breaker *modfraud.ServiceBreaker[*das.DASer, *header.ExtendedHeader]) error {
				return breaker.Stop(ctx)
			}),
		)),
		// Module is needed for the RPC handler
		fx.Provide(func(das *das.DASer) Module {
			return das
		}),
	)
}
```

Note the guard at the top: if DAS is disabled or this is a Bridge node, a stub is provided instead. The `ServiceBreaker` pattern appears throughout the codebase -- it wraps a service so that if a fraud proof is detected, the service is automatically stopped to protect the node from acting on fraudulent data.

## 14. Blob Service: blob/service.go

The Blob service is the user-facing layer for submitting and retrieving blobs -- the actual application data stored in Celestia blocks. It orchestrates between the state module (for submitting transactions), share getter (for retrieving data), and header service (for looking up blocks).

```bash
sed -n '52,78p' blob/service.go
```

```output
type Service struct {
	// ctx represents the Service's lifecycle context.
	ctx    context.Context
	cancel context.CancelFunc
	// accessor dials the given celestia-core endpoint to submit blobs.
	blobSubmitter Submitter
	// shareGetter retrieves the EDS to fetch all shares from the requested header.
	shareGetter shwap.Getter
	// headerGetter fetches header by the provided height
	headerGetter func(context.Context, uint64) (*header.ExtendedHeader, error)
	// headerSub subscribes to new headers to supply to blob subscriptions.
	headerSub func(ctx context.Context) (<-chan *header.ExtendedHeader, error)
}

func NewService(
	submitter Submitter,
	getter shwap.Getter,
	headerGetter func(context.Context, uint64) (*header.ExtendedHeader, error),
	headerSub func(ctx context.Context) (<-chan *header.ExtendedHeader, error),
) *Service {
	return &Service{
		blobSubmitter: submitter,
		shareGetter:   getter,
		headerGetter:  headerGetter,
		headerSub:     headerSub,
	}
}
```

The key operations are:

- **Submit** -- validates blob namespaces, converts to `libshare.Blob`, and calls `SubmitPayForBlob` via the state module to create a PayForBlob transaction
- **Get** -- retrieves a specific blob by height, namespace, and commitment by fetching the EDS namespace data and matching commitments
- **GetAll** -- retrieves all blobs under given namespaces at a height (parallelized across namespaces)
- **Subscribe** -- streams new blobs matching a namespace as blocks arrive
- **Included** -- verifies a blob was included at a specific height by recomputing the commitment and comparing proofs

Let's look at how Submit works:

```bash
sed -n '166,187p' blob/service.go
```

```output
// Submit sends PFB transaction and reports the height at which it was included.
// Allows sending multiple Blobs atomically synchronously.
// Uses default wallet registered on the Node.
// Handles gas estimation and fee calculation.
func (s *Service) Submit(ctx context.Context, blobs []*Blob, txConfig *SubmitOptions) (uint64, error) {
	log.Debugw("submitting blobs", "amount", len(blobs))

	libBlobs := make([]*libshare.Blob, len(blobs))
	for i := range blobs {
		if err := blobs[i].Namespace().ValidateForBlob(); err != nil {
			return 0, fmt.Errorf("not allowed namespace %s were used to build the blob", blobs[i].Namespace().ID())
		}

		libBlobs[i] = blobs[i].Blob
	}

	resp, err := s.blobSubmitter.SubmitPayForBlob(ctx, libBlobs, txConfig)
	if err != nil {
		return 0, err
	}
	return uint64(resp.Height), nil
}
```

And the blob module's DI wiring shows how it pulls dependencies from the header and state modules:

```bash
cat nodebuilder/blob/module.go
```

```output
package blob

import (
	"context"

	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/header"
	headerService "github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

func ConstructModule() fx.Option {
	return fx.Module("blob",
		fx.Provide(
			func(service headerService.Module) func(context.Context, uint64) (*header.ExtendedHeader, error) {
				return service.GetByHeight
			},
		),
		fx.Provide(
			func(service headerService.Module) func(context.Context) (<-chan *header.ExtendedHeader, error) {
				return service.Subscribe
			},
		),
		fx.Provide(fx.Annotate(
			func(
				state state.Module,
				sGetter shwap.Getter,
				getByHeightFn func(context.Context, uint64) (*header.ExtendedHeader, error),
				subscribeFn func(context.Context) (<-chan *header.ExtendedHeader, error),
			) *blob.Service {
				return blob.NewService(state, sGetter, getByHeightFn, subscribeFn)
			},
			fx.OnStart(func(ctx context.Context, serv *blob.Service) error {
				return serv.Start(ctx)
			}),
			fx.OnStop(func(ctx context.Context, serv *blob.Service) error {
				return serv.Stop(ctx)
			}),
		)),
		fx.Provide(func(serv *blob.Service) Module {
			return serv
		}),
	)
}
```

Notice how the blob module doesn't take a node type -- it's the same for all types. The differences in behavior come from the injected dependencies: a Light node's `shwap.Getter` fetches from the network, while a Bridge node's fetches from the local store.

## 15. State Service: nodebuilder/state/module.go

The State module manages on-chain state interaction -- balance queries, transaction submission, gas estimation. It connects to a Celestia Core node's gRPC endpoint.

```bash
sed -n '23,59p' nodebuilder/state/module.go
```

```output
func ConstructModule(tp node.Type, cfg *Config, coreCfg *core.Config) fx.Option {
	// sanitize config values before constructing module
	cfgErr := cfg.Validate()
	baseComponents := fx.Options(
		fx.Supply(*cfg),
		fx.Error(cfgErr),
		fx.Provide(func(ks keystore.Keystore) (keyring.Keyring, AccountName, error) {
			return Keyring(*cfg, ks)
		}),
		fxutil.ProvideIf(coreCfg.IsEndpointConfigured(), fx.Annotate(
			coreAccessor,
			fx.OnStart(func(ctx context.Context,
				breaker *modfraud.ServiceBreaker[*state.CoreAccessor, *header.ExtendedHeader],
			) error {
				return breaker.Start(ctx)
			}),
			fx.OnStop(func(ctx context.Context,
				breaker *modfraud.ServiceBreaker[*state.CoreAccessor, *header.ExtendedHeader],
			) error {
				return breaker.Stop(ctx)
			}),
		)),
		fxutil.ProvideIf(!coreCfg.IsEndpointConfigured(), func() (*state.CoreAccessor, Module) {
			return nil, &stubbedStateModule{}
		}),
	)

	switch tp {
	case node.Light, node.Full, node.Bridge:
		return fx.Module(
			"state",
			baseComponents,
		)
	default:
		panic("invalid node type")
	}
}
```

The state module uses a clever conditional pattern: `fxutil.ProvideIf`. If a Core endpoint is configured, it creates a real `CoreAccessor` (wrapped in a fraud ServiceBreaker). If not, it provides a stub that returns errors -- this allows Light nodes to run without a Core connection, just without state query capabilities.

The `CoreAccessor` is also fraud-protected: if a fraud proof is detected, state operations are halted to prevent the node from submitting transactions based on potentially fraudulent data.

## 16. Fraud Detection: nodebuilder/fraud/module.go

The fraud module provides the infrastructure for detecting, creating, and propagating fraud proofs. Different node types handle fraud differently based on their sync requirements.

```bash
cat nodebuilder/fraud/module.go
```

```output
package fraud

import (
	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/fx"

	"github.com/celestiaorg/go-fraud"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

var log = logging.Logger("module/fraud")

func ConstructModule(tp node.Type) fx.Option {
	baseComponent := fx.Options(
		fx.Provide(Unmarshaler),
		fx.Provide(func(serv fraud.Service[*header.ExtendedHeader]) fraud.Getter[*header.ExtendedHeader] {
			return serv
		}),
	)
	switch tp {
	case node.Light:
		return fx.Module(
			"fraud",
			baseComponent,
			fx.Provide(newFraudServiceWithSync),
		)
	case node.Full, node.Bridge:
		return fx.Module(
			"fraud",
			baseComponent,
			fx.Provide(newFraudServiceWithoutSync),
		)
	default:
		panic("invalid node type")
	}
}
```

- **Light** nodes use `newFraudServiceWithSync` -- they wait for header sync to complete before processing fraud proofs, because they need a consistent view of the chain to validate proofs
- **Full/Bridge** nodes use `newFraudServiceWithoutSync` -- they process fraud proofs asynchronously without waiting for sync, since they have complete local data

The `ServiceBreaker` pattern we've seen in the header syncer, DASer, and state accessor all tie back to this fraud service. When a fraud proof is received and validated, the breaker trips and stops those services.

## 17. RPC API Server: api/rpc/server.go

The RPC server exposes all node functionality over a JSON-RPC 2.0 API with JWT-based authentication. This is how external tools and applications interact with the node.

```bash
sed -n '30,67p' api/rpc/server.go
```

```output
type Server struct {
	srv          *http.Server
	rpc          *jsonrpc.RPCServer
	listener     net.Listener
	authDisabled bool

	started    atomic.Bool
	corsConfig CORSConfig

	signer   jwt.Signer
	verifier jwt.Verifier
}

func NewServer(
	address, port string,
	authDisabled bool,
	corsConfig CORSConfig,
	signer jwt.Signer,
	verifier jwt.Verifier,
) *Server {
	rpc := jsonrpc.NewServer()
	srv := &Server{
		rpc:          rpc,
		signer:       signer,
		verifier:     verifier,
		authDisabled: authDisabled,
		corsConfig:   corsConfig,
	}

	srv.srv = &http.Server{
		Addr:    net.JoinHostPort(address, port),
		Handler: srv.newHandlerStack(rpc),
		// the amount of time allowed to read request headers. set to the default 2 seconds
		ReadHeaderTimeout: 2 * time.Second,
	}

	return srv
}
```

The handler stack is layered:

1. **JSON-RPC server** (`filecoin-project/go-jsonrpc`) at the core
2. **Auth middleware** -- extracts JWT tokens, verifies them, and maps to permission levels (read/write/admin)
3. **CORS middleware** -- configurable cross-origin resource sharing

When auth is disabled (e.g., for development), all permissions are granted and CORS allows all origins.

```bash
sed -n '119,127p' api/rpc/server.go
```

```output
func (s *Server) RegisterService(namespace string, service, out any) {
	if s.authDisabled {
		s.rpc.Register(namespace, service)
		return
	}

	auth.PermissionedProxy(perms.AllPerms, perms.DefaultPerms, service, getInternalStruct(out))
	s.rpc.Register(namespace, out)
}
```

Each module registers its methods under a namespace (e.g., `blob`, `header`, `state`, `das`, `share`, `node`, `p2p`). The `PermissionedProxy` call wraps each method with permission checks -- methods are tagged with required permission levels, and the JWT token's permissions must match.

The RPC module wiring connects everything:

```bash
cat nodebuilder/rpc/module.go
```

```output
package rpc

import (
	"context"

	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/api/rpc"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

func ConstructModule(tp node.Type, cfg *Config) fx.Option {
	// sanitize config values before constructing module
	cfgErr := cfg.Validate()

	baseComponents := fx.Options(
		fx.Supply(cfg),
		fx.Error(cfgErr),
		fx.Provide(fx.Annotate(
			server,
			fx.OnStart(func(ctx context.Context, server *rpc.Server) error {
				return server.Start(ctx)
			}),
			fx.OnStop(func(ctx context.Context, server *rpc.Server) error {
				return server.Stop(ctx)
			}),
		)),
	)

	switch tp {
	case node.Light, node.Full, node.Bridge:
		return fx.Module(
			"rpc",
			baseComponents,
			fx.Invoke(registerEndpoints),
		)
	default:
		panic("invalid node type")
	}
}
```

The `registerEndpoints` function (invoked by fx at startup) takes all the module interfaces and registers their methods on the RPC server under the appropriate namespaces.

## 18. Pruning System: nodebuilder/pruner/module.go

The pruner manages storage lifecycle by removing old data that is no longer needed. Different node types have different pruning behaviors based on their availability windows.

```bash
sed -n '22,86p' nodebuilder/pruner/module.go
```

```output
func ConstructModule(tp node.Type) fx.Option {
	prunerService := fx.Options(
		fx.Provide(fx.Annotate(
			newPrunerService,
			fx.OnStart(func(ctx context.Context, p *pruner.Service) error {
				return p.Start(ctx)
			}),
			fx.OnStop(func(ctx context.Context, p *pruner.Service) error {
				return p.Stop(ctx)
			}),
		)),
		// This is necessary to invoke the pruner service as independent thanks to a
		// quirk in FX.
		fx.Invoke(func(_ *pruner.Service) {}),
	)

	baseComponents := fx.Options(
		// supply the default config, which can only be overridden by
		// passing the `--archival` flag
		fx.Supply(DefaultConfig()),
		// TODO @renaynay: move this to share module construction
		advertiseArchival(),
		prunerService,
	)

	switch tp {
	case node.Light:
		// LNs enforce pruning by default
		return fx.Module("prune",
			baseComponents,
			fx.Supply(modshare.Window(availability.SamplingWindow)),
			// TODO(@walldiss @renaynay): remove conversion after Availability and Pruner interfaces are merged
			//  note this provide exists in pruner module to avoid cyclical imports
			fx.Provide(func(la *light.ShareAvailability) pruner.Pruner { return la }),
		)
	case node.Full:
		return fx.Module("prune",
			baseComponents,
			fx.Supply(modshare.Window(availability.StorageWindow)),
			fx.Provide(func(cfg *Config) []fullavail.Option {
				if cfg.EnableService {
					return make([]fullavail.Option, 0)
				}
				return []fullavail.Option{fullavail.WithArchivalMode()}
			}),
			fx.Provide(func(fa *fullavail.ShareAvailability) pruner.Pruner { return fa }),
			fx.Invoke(convertToPruned),
		)
	case node.Bridge:
		return fx.Module("prune",
			baseComponents,
			fx.Provide(func(cfg *Config) ([]core.Option, []fullavail.Option) {
				if cfg.EnableService {
					return make([]core.Option, 0), make([]fullavail.Option, 0)
				}
				return []core.Option{core.WithArchivalMode()}, []fullavail.Option{fullavail.WithArchivalMode()}
			}),
			fx.Provide(func(fa *fullavail.ShareAvailability) pruner.Pruner { return fa }),
			fx.Supply(modshare.Window(availability.StorageWindow)),
			fx.Invoke(convertToPruned),
		)
	default:
		panic("unknown node type")
	}
}
```

Pruning behavior by node type:

- **Light**: Uses a `SamplingWindow` -- only keeps data long enough to sample it, then prunes. This is the most aggressive pruning since Light nodes only need statistical verification.
- **Full**: Uses a `StorageWindow` -- keeps data longer since Full nodes serve data to the network. Can run in **archival mode** (disabled pruning) via the `--archival` flag.
- **Bridge**: Same storage window as Full, can also run archival. Bridge nodes that switch from archival to pruned mode trigger a `convertToPruned` hook that resets the pruning checkpoint to clean up all old data.

The `advertiseArchival()` function controls peer discovery: archival nodes (non-pruning Full/Bridge) advertise themselves so peers know they can serve historical data.

## 19. Node Lifecycle: Start, Run, Stop

With all modules wired up, the Node's lifecycle is straightforward. The `Start` method calls fx's app.Start (which triggers all `OnStart` hooks in dependency order), then prints the node's identity and listening addresses.

```bash
sed -n '102,136p' nodebuilder/node.go
```

```output
// Start launches the Node and all its components and services.
func (n *Node) Start(ctx context.Context) error {
	to := n.Config.Node.StartupTimeout
	ctx, cancel := context.WithTimeout(ctx, to)
	defer cancel()

	err := n.start(ctx)
	if err != nil {
		log.Debugf("error starting %s Node: %s", n.Type, err)
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("node: failed to start within timeout(%s): %w", to, err)
		}
		return fmt.Errorf("node: failed to start: %w", err)
	}

	log.Infof("\n\n/_____/  /_____/  /_____/  /_____/  /_____/ \n\n"+
		"Started celestia DA node \n"+
		"node version: 	%s\nnode type: 	%s\nnetwork: 	%s\n\n"+
		"/_____/  /_____/  /_____/  /_____/  /_____/ \n",
		node.GetBuildInfo().SemanticVersion,
		strings.ToLower(n.Type.String()),
		n.Network)

	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(n.Host))
	if err != nil {
		log.Errorw("Retrieving multiaddress information", "err", err)
		return err
	}
	fmt.Println("The p2p host is listening on:")
	for _, addr := range addrs {
		fmt.Println("* ", addr.String())
	}
	fmt.Println()
	return nil
}
```

And the Stop method provides graceful shutdown with a configurable timeout:

```bash
sed -n '150,169p' nodebuilder/node.go
```

```output
// Stop shuts down the Node, all its running Modules/Services and returns.
// Canceling the given context earlier 'ctx' unblocks the Stop and aborts graceful shutdown forcing
// remaining Modules/Services to close immediately.
func (n *Node) Stop(ctx context.Context) error {
	to := n.Config.Node.ShutdownTimeout
	ctx, cancel := context.WithTimeout(ctx, to)
	defer cancel()

	err := n.stop(ctx)
	if err != nil {
		log.Debugf("error stopping %s Node: %s", n.Type, err)
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("node: failed to stop within timeout(%s): %w", to, err)
		}
		return fmt.Errorf("node: failed to stop: %w", err)
	}

	log.Debugf("stopped %s Node", n.Type)
	return nil
}
```

Both Start and Stop use configurable timeouts. The stop sequence is the reverse of start -- fx calls all `OnStop` hooks in reverse dependency order, ensuring components that depend on other components are stopped first. The DASer saves its sampling checkpoint, the header syncer stops syncing, the P2P host closes connections, and the EDS store flushes its cache.

## 20. Putting It All Together

Here's the complete data flow for each node type:

### Bridge Node

    Celestia Core (consensus) --gRPC--> BlockFetcher --> Listener
      |
      +--> Constructs ExtendedHeader (header + commit + validators + DAH)
      +--> Stores EDS in Store (ODS + Q4 files)
      +--> Broadcasts header via P2P PubSub
      +--> Broadcasts data hash via ShrExSub
      |
      +--> Serves header requests via ExchangeServer
      +--> Serves share requests via ShrEx Server
      +--> Exposes all operations via RPC API

### Light Node

    P2P Network --PubSub--> Header Subscriber --> Header Syncer --> Header Store
      |
      +--> DASer subscribes to new headers
      +--> Randomly samples shares via ShrEx Client / Bitswap
      +--> Validates samples via light.ShareAvailability
      +--> Broadcasts fraud proofs if byzantine behavior detected
      |
      +--> Blob service retrieves blobs by fetching namespace data
      +--> State service queries/submits via optional Core connection
      +--> Exposes all operations via RPC API

### Full Node

    P2P Network --PubSub--> Header Subscriber --> Header Syncer --> Header Store
      |
      +--> Downloads complete EDS via ShrEx Client / Bitswap
      +--> Stores EDS in Store (ODS + Q4 files)
      +--> Validates via full.ShareAvailability
      |
      +--> Serves share requests via ShrEx Server
      +--> DASer validates availability (same as Light)
      +--> Blob service, State service, RPC API (same as Light)

## Key Architectural Patterns

1. **uber/fx Dependency Injection** -- The entire node is assembled declaratively. Modules register providers and lifecycle hooks; fx resolves the graph and manages startup/shutdown order.

2. **Node Type Polymorphism** -- Rather than separate codebases, each module's `ConstructModule` switches on `node.Type` to configure type-specific behavior. The same binary serves all three roles.

3. **ServiceBreaker (Fraud Protection)** -- Critical services (syncer, DASer, state accessor) are wrapped in `ServiceBreaker`s that automatically stop the service when fraud is detected.

4. **Modular Getter Pattern** -- Data retrieval is abstracted behind the `shwap.Getter` interface. Bridge/Full nodes get a getter backed by local storage; Light nodes get one that fetches from the network. Higher-level services (blob, DAS) are unaware of the source.

5. **Checkpoint-based Resumption** -- The DASer and pruner persist checkpoints to BadgerDB, allowing them to resume from where they left off after a restart rather than reprocessing the entire chain.

6. **Dual Storage Strategy** -- A BadgerDB key-value store for metadata/checkpoints/headers and flat files (ODS/Q4) for the large EDS data, with filesystem hard links providing O(1) height-to-hash lookups.
