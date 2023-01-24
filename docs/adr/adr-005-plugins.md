# ADR 005: Plugins

## Changelog

- 2022-02-06: init commit
- 2022-03-04: decision

## Context

The modular design of celestia-node allows for the functionality it provides to be extended. The main blocker for creating custom light client applications is having a mechanism to access the internals of a celestia-node. Here we are suggesting that there be some interface that installs custom services into celestia-node, along with making the celestia-node cli more portable, so that light client developers can create their own binaries with the custom services that they create.

## Alternative Approaches

### Export most if not all the fields of a celestia-node object

It should be possible to achieve the flexibility that a custom node requires, but it limits the UX for both developers and custom node users. For instance, running the custom node takes two binaries, both of which have to be installed using the associated versions. When creating a custom node, we have to implement redundant functionality, such as basic program structure, clis, and some tests.

It also makes it more difficult for custom nodes to support different types of celestia nodes. With the plugin design, it is possible to run the same plugin on every node type without adding any code.

Lastly, this approach wouldn't make it easy to run custom nodes with multiple modifications, whereas the plugin approach does.

### Expose more services over rpc

This would likely require a lot of overhead, and not eliminate any of the existing complexity.

## Decision

Postponed for now. Per these comments, [comment1](https://github.com/celestiaorg/celestia-node/pull/414#issuecomment-1055871523) [comment2](https://github.com/celestiaorg/celestia-node/pull/414#discussion_r817744441), we are planning on taking a different approach that focuses on exposing a general purpose API. While this will not provide the custom functionality that plugins provide, it will be less cumbersome to support in the future while also serving our most of our needs. However, there is still a possibility that we merge the Plugin implementation, or a different design that provides similar functionality, in the future should we decide to support custom nodes.

## Detailed Design

This approach features a new `Plugin` interface.

```go
type Plugin interface {
   Name() string
   Initialize(path string) error
   Components(cfg *Config, store Store) fxutil.Option
}
```

The implementations of the plugin interface can then be passed to new functions that generate and return the provided `Plugin`s.

```go
// NewRootCmd returns an initiated celestia root command. The provided plugins
// will be installed into celestia-node.
func NewRootCmd(plugs ...node.Plugin) *cobra.Command {
   plugins := make([]string, len(plugs))
   for i, plug := range plugs {
       plugins[i] = fmt.Sprintf("with plugin: %s", plug.Name())
   }
   ...
   ...
   command.AddCommand(
       NewBridgeCommand(plugs), // <--
       NewLightCommand(plugs),  // <--
       versionCmd,
   )
   command.SetHelpCommand(&cobra.Command{})
   return command
}
```

```go
// NewBridgeCommand creates a new bridge sub command. Provided plugins are
// installed into celestia-node
func NewBridgeCommand(plugs []node.Plugin) *cobra.Command {
   command := &cobra.Command{
       Use:   "bridge [subcommand]",
       Args:  cobra.NoArgs,
       Short: "Manage your Bridge node",
       PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
           ...
       },
   }

   command.AddCommand(
       Init(
           plugs, // <--
           ...
       ),
       Start(
           plugs, // <--
           ...
       ),
   )

   return command
}
```

When starting or initiating a new celestia node, the plugins are passed as settings

```go
// settings store all the non Config values that can be altered for Node with Options.
type settings struct {
   P2PKey     crypto.PrivKey
   Host       p2p.HostBase
   CoreClient core.Client
   Plugins    []Plugin // <--
}
```

Here the plugins are being added as their own services when creating a celestia node

```go
// WithPlugins adds the provided plugins to the settings
func WithPlugins(plugins ...Plugin) Option {
   return func(c *Config, s *settings) error {
       s.Plugins = plugins
       return nil
   }
}

// New assembles a new Node with the given type 'tp' over Store 'store'.
func New(tp Type, store Store, options ...Option) (*Node, error) {
   cfg, err := store.Config()
   if err != nil {
       return nil, err
   }

   s := new(settings)
   for _, option := range options {
       if option != nil {
           err := option(cfg, s)
           if err != nil {
               return nil, err
           }
       }
   }

   switch tp {
   case Bridge:
       return newNode(bridgeComponents(cfg, store), s.plugins(cfg, store), s.overrides()) // <--
   case Light:
       return newNode(lightComponents(cfg, store), s.plugins(cfg, store), s.overrides())  // <--
   default:
       panic("node: unknown Node Type")
   }
}
```

The plugins can also perform arbitrary initialization routines

```go
// Init initializes the Node FileSystem Store for the given Node Type 'tp' in the directory under 'path' with
// default Config. Options are applied over default Config and persisted on disk.
func Init(path string, tp Type, options ...Option) error {
   cfg, sets := DefaultConfig(tp), new(settings)
   for _, option := range options {
       if option != nil {
           err := option(cfg, sets)
           if err != nil {
               return err
           }
       }
   }

   ...

   for _, plug := range sets.Plugins {
       err = plug.Initialize(path)
       if err != nil {
           return err
       }
   }

  ...

   return nil
}
```

### What are the user requirements?

The plugin design works by utilizing the `uber/fx` dependency injection framework. This works by first combining all the components that return a `PluginResult` to a `RootPlugin` type to the `Node` struct.

```go

// Node represents the core structure of a Celestia node. It keeps references to all Celestia-specific
type Node struct {
   ...

   // RootPlugin serves as an arbitrary type that is used to collect the return
   // values of plugins
   RootPlugin
   ...
}

// RootPlugin strictly serves as a type that composes the Node struct. This
// provides plugins a way to force fx to load the desired plugin components
type RootPlugin struct{}

type PluginResult interface{}

// this function is used to collect multiple plugin components
func collectSubOutlets(s ...PluginResult) RootPlugin {
   return RootPlugin{}
}

func collectComponents() fxutil.Option {
   return fxutil.Raw(
       fx.Provide(
           fx.Annotate(
               collectSubOutlets,
               fx.ParamTags(`group:"plugins"`),
           ),
       ),
   )
}
```

When creating a plugin, at least one of the components must return a `PluginResult` type.

```go
// use the PluginResult to force fx to call this function
func newPluginService() PluginResult {
   initiallyEmpty = testStr
   return struct{}{}
}
```

Also, the user must annotate this fxutil.Option with the `"plugins"` group tag

```go
func (plug *testPlugin) Components(cfg *Config, store Store) fxutil.Option {
   return fxutil.Raw(
       fx.Provide(
           fx.Annotate(                          // <--
               newPluginService,                 // <--
               fx.ResultTags(`group:"plugins"`), // <--
           ),
       ),
   )
}
```

> - How will the changes be tested?

please see tests in the implementation PR [#407 tests](https://github.com/celestiaorg/celestia-node/blob/8416bcc5e414525e337904afbd87e20beddb50fd/node/plugin_test.go)
along with the refactored version of optimint's dalc [#55](https://github.com/celestiaorg/dalc/pull/55)

> - Will these changes require a breaking (major) release?

They should not.

> - Does this change require coordination with the Celestia fork of the SDK, celestia-app/-core, or any other celestiaorg repository?

Yes, this change will dramatically affect how optimint's dalc works

## Status

Proposed

## Consequences

### Positive

- easier to create custom applications that run on top of celestia
- allows for developers to create a better UX for their custom celestia-nodes
- isolates the added functionality to its own service(s), which could potentially be combined with other plugins
- helps move us towards our goal of reducing any duplicate functionality coded in optimint's dalc

### Negative

- adds some complexity
- it will likely add future engineering efforts if we continue to support this feature
- perhaps less intuitive for those not familiar with uber/fx

## References

- blocking a refactor of dalc [#55](https://github.com/celestiaorg/dalc/pull/55)
- first discussed and current implementation [#407](https://github.com/celestiaorg/celestia-node/pull/407)
- initial issue [#406](https://github.com/celestiaorg/celestia-node/issues/406)
