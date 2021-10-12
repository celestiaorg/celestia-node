package node

// Type defines the Node type (e.g. `light`, `full`) for identity purposes.
// The zero value for Type is invalid.
type Type uint8

const (
	// Full is a full-featured Celestia Node.
	Full Type = iota + 1
	// Light is a stripped-down Celestia Node which aims to be lightweight while preserving highest possible
	// security guarantees.
	Light
	// Dev is a full-featured Celestia Node running alongside a mock embedded Core node process,
	// designed to simulate block production (for testing/dev purposes only).
	Dev
)

// String converts Type to its string representation.
func (t Type) String() string {
	if !t.IsValid() {
		return "unknown"
	}
	return typeToString[t]
}

// IsValid reports whether the Type is valid.
func (t Type) IsValid() bool {
	_, ok := typeToString[t]
	return ok
}

// ParseType converts string in a type if possible.
func ParseType(str string) Type {
	tp, ok := stringToType[str]
	if !ok {
		return 0
	}

	return tp
}

// typeToString keeps string representations of all valid Types.
var typeToString = map[Type]string{
	Full:  "Full",
	Light: "Light",
	Dev:   "Dev",
}

// typeToString maps strings representations of all valid Types.
var stringToType = map[string]Type{
	"Full":  Full,
	"Light": Light,
	"Dev":   Dev,
}
