package share

// Parameterizable defines interface for parameters configuration
type Parameterizable interface {
	// SetParam sets a configuration parameter key to the provided value.
	// This is primarily used to allow functional options to be generic
	// for all implementations of the Availability interface
	SetParam(key string, value any)
}
