package share

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	semanticVersion string // Semantic Version of the application
	lastCommit      string // Last commit of the application
	buildTime       string // Build time of the application
	systemVersion   string // System version of the application
	golangVersion   string // Go programming language version
)

// SetSemanticVersion sets the value of the semanticVersion variable
func SetSemanticVersion(version string) {
	semanticVersion = version
}

// GetSemanticVersion returns the value of the semanticVersion variable
func GetSemanticVersion() string {
	return semanticVersion
}

// SetLastCommit sets the value of the lastCommit variable
func SetLastCommit(commit string) {
	lastCommit = commit
}

// GetLastCommit returns the value of the lastCommit variable
func GetLastCommit() string {
	return lastCommit
}

// SetBuildTime sets the value of the buildTime variable
func SetBuildTime(btime string) {
	buildTime = btime
}

// GetBuildTime returns the value of the buildTime variable
func GetBuildTime() string {
	return buildTime
}

// SetSystemVersion sets the value of the systemVersion variable
func SetSystemVersion(version string) {
	systemVersion = version
}

// GetSystemVersion returns the value of the systemVersion variable
func GetSystemVersion() string {
	return systemVersion
}

// SetGolangVersion sets the value of the golangVersion variable
func SetGolangVersion(version string) {
	golangVersion = version
}

// GetGolangVersion returns the value of the golangVersion variable
func GetGolangVersion() string {
	return golangVersion
}

// SetMetricsValues sets values to all variables
func SetMetricsValues(
	lastCommit,
	semanticVersion,
	systemVersion,
	golangVersion,
	buildTime string) {
	SetSemanticVersion(semanticVersion)
	SetLastCommit(lastCommit)
	SetBuildTime(buildTime)
	SetSystemVersion(systemVersion)
	SetGolangVersion(golangVersion)
}

// RegisterPromMetrics adds all the info metrics to Prometheus
func RegisterPromMetrics(registerer prometheus.Registerer) prometheus.Registerer {
	metrics := []struct {
		name string
		help string
	}{
		{
			name: "celestia_node_version",
			help: "Semantic version of Celestia Node",
		},
		{
			name: "celestia_node_last_commit",
			help: "Last commit of Celestia Node",
		},
		{
			name: "celestia_node_build_time",
			help: "Build time of Celestia Node",
		},
		{
			name: "celestia_node_system_version",
			help: "System version of Celestia Node",
		},
		{
			name: "celestia_node_golang_version",
			help: "Go version of Celestia Node",
		},
	}

	for _, metric := range metrics {
		m := prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: metric.name,
				Help: metric.help,
			},
			[]string{metric.name},
		)

		registerer.MustRegister(m)
		m.WithLabelValues(getMetricValue(metric.name)).Set(1)
	}

	return registerer
}

// getMetricValue returns the values
func getMetricValue(name string) string {
	switch name {
	case "celestia_node_version":
		return GetSemanticVersion()
	case "celestia_node_last_commit":
		return GetLastCommit()
	case "celestia_node_build_time":
		return GetBuildTime()
	case "celestia_node_system_version":
		return GetSystemVersion()
	case "celestia_node_golang_version":
		return GetGolangVersion()
	default:
		return ""
	}
}
