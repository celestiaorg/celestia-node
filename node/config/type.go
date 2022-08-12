package config

// NodeType defines the Node type (e.g. `light`, `bridge`) for identity purposes.
// The zero value for NodeType is invalid.
type NodeType uint8

const (
	// Bridge is a Celestia Node that bridges the Celestia consensus network and data availability network.
	// It maintains a trusted channel/connection to a Celestia Core node via the core.Client API.
	Bridge NodeType = iota + 1
	// Light is a stripped-down Celestia Node which aims to be lightweight while preserving the highest possible
	// security guarantees.
	Light
	// Full is a Celestia Node that stores blocks in their entirety.
	Full
)

// String converts NodeType to its string representation.
func (t NodeType) String() string {
	if !t.IsValid() {
		return "unknown"
	}
	return typeToString[t]
}

// IsValid reports whether the NodeType is valid.
func (t NodeType) IsValid() bool {
	_, ok := typeToString[t]
	return ok
}

// ParseType converts string in a type if possible.
func ParseType(str string) NodeType {
	tp, ok := stringToType[str]
	if !ok {
		return 0
	}

	return tp
}

// typeToString keeps string representations of all valid Types.
var typeToString = map[NodeType]string{
	Bridge: "Bridge",
	Light:  "Light",
	Full:   "Full",
}

// typeToString maps strings representations of all valid Types.
var stringToType = map[string]NodeType{
	"Bridge": Bridge,
	"Light":  Light,
	"Full":   Full,
}
