package node

// BuildInfo stores all necessary information for the current build.
type BuildInfo struct {
	LastCommit      string
	SemanticVersion string
	SystemVersion   string
	GolangVersion   string
}
