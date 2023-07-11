package share

var SemanticVersion string
var LastCommit string

// SetSemanticVersion sets the value of the SemanticVersion variable
func SetSemanticVersion(version string) {
	SemanticVersion = version
}

// GetSemanticVersion returns the value of the SemanticVersion variable
func GetSemanticVersion() string {
	return SemanticVersion
}

// SetLastCommit sets the value of the LastCommit variable
func SetLastCommit(commit string) {
	LastCommit = commit
}

// GetLastCommit returns the value of the LastCommit variable
func GetLastCommit() string {
	return LastCommit
}
