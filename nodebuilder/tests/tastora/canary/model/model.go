package model

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"
)

type Outcome string

const (
	Pass         Outcome = "pass"
	Fail         Outcome = "fail"
	Unsupported  Outcome = "unsupported"
	Inconclusive Outcome = "inconclusive"
)

// DAS job types, as the das logger reports them.
const (
	JobRecent  = "recent"
	JobCatchup = "catchup"
	JobRetry   = "retry"
)

// Profile names. Mainnet and Mocha are the public networks; Local targets a
// private network and is the only profile that accepts overrides.
const (
	ProfileMainnet = "mainnet"
	ProfileMocha   = "mocha"
	ProfileLocal   = "local"
)

// OfficialImageRepository is the only registry path admitted for public
// network digests; any other image runs by its exact local image ID.
const OfficialImageRepository = "ghcr.io/celestiaorg/celestia-node"

// EvidenceNativeLogs marks a witness derived from the node's own sampling logs
// joined with an independently validated header.
const EvidenceNativeLogs = "native_logs"

type Check struct {
	Name            string         `json:"name"`
	Outcome         Outcome        `json:"outcome"`
	Code            string         `json:"code,omitempty"`
	Detail          string         `json:"detail,omitempty"`
	DurationSeconds float64        `json:"duration_seconds"`
	Evidence        map[string]any `json:"evidence,omitempty"`
}
type Profile struct {
	Name            string `json:"name"`
	Network         string `json:"network"`
	ChainID         string `json:"chain_id"`
	Image           string `json:"image"`
	SourceCommit    string `json:"source_commit"`
	TelemetrySchema string `json:"telemetry_schema"`
	// Release is the tag the image digest was resolved from. It is for readers;
	// the digest and the source commit identify the image.
	Release               string   `json:"release,omitempty"`
	CustomNetwork         string   `json:"custom_network,omitempty"`
	Bootstrappers         []string `json:"bootstrappers"`
	SampleAmount          int      `json:"sample_amount"`
	SamplingWindowSeconds int64    `json:"sampling_window_seconds"`
	StorageWindowSeconds  int64    `json:"storage_window_seconds"`
}
type HeaderRef struct {
	Height  uint64    `json:"height"`
	Hash    string    `json:"hash"`
	Root    string    `json:"root"`
	ChainID string    `json:"chain_id"`
	Time    time.Time `json:"time"`
	Width   int       `json:"width"`
}

// Witness is one sampling completion bound to a validated header.
type Witness struct {
	// Check names the check this witness satisfies: das, das_recent or restart.
	Check       string    `json:"check"`
	Header      HeaderRef `json:"header"`
	Epoch       int       `json:"epoch"`
	JobType     string    `json:"job_type"`
	SampleCount int       `json:"sample_count"`
	// Empty marks a DAS worker pass over an empty data square: nothing to
	// sample, no session, SampleCount 0.
	Empty           bool      `json:"empty_block,omitempty"`
	StartedAt       time.Time `json:"started_at"`
	CompletedAt     time.Time `json:"completed_at"`
	DurationSeconds float64   `json:"duration_seconds"`
	EvidenceKind    string    `json:"evidence_kind"`
}
type ProcessInfo struct {
	ContainerID string    `json:"container_id"`
	VolumeID    string    `json:"volume_id"`
	ImageID     string    `json:"image_id"`
	StartedAt   time.Time `json:"started_at"`
	FinishedAt  time.Time `json:"finished_at"`
	Running     bool      `json:"running"`
	ExitCode    int       `json:"exit_code"`
	OOMKilled   bool      `json:"oom_killed"`
	ForcedKill  bool      `json:"forced_kill"`
}
type StoreIdentity struct {
	VolumeID string `json:"volume_id"`
	Owner    string `json:"owner"`
	Fresh    bool   `json:"fresh"`
}

// Metrics are wall-clock observations measured from the node's process start
// (or, for the restart metric, from the restarted process start). Nil means
// the phase did not complete.
type Metrics struct {
	StartupSeconds       *float64 `json:"startup_seconds,omitempty"`
	HeaderSyncSeconds    *float64 `json:"header_sync_seconds,omitempty"`
	FirstSampleSeconds   *float64 `json:"first_sample_seconds,omitempty"`
	RecentSampleSeconds  *float64 `json:"recent_sample_seconds,omitempty"`
	RestartResumeSeconds *float64 `json:"restart_resume_seconds,omitempty"`
	// BootstrappersConnected counts the network's bootstrap peers that
	// were connected once the node was ready, out of BootstrappersTotal.
	BootstrappersConnected *int `json:"bootstrappers_connected,omitempty"`
	BootstrappersTotal     *int `json:"bootstrappers_total,omitempty"`
	// SyncHeadersPerSecond is the header sync rate over the first process
	// lifetime: headers stored above the initial tail per second of uptime.
	SyncHeadersPerSecond *float64 `json:"sync_headers_per_second,omitempty"`
}

type Result struct {
	SchemaVersion string    `json:"schema_version"`
	RunID         string    `json:"run_id"`
	Profile       Profile   `json:"profile"`
	NodeID        string    `json:"node_id,omitempty"`
	ImageID       string    `json:"image_id,omitempty"`
	StartedAt     time.Time `json:"started_at"`
	FinishedAt    time.Time `json:"finished_at"`
	Outcome       Outcome   `json:"outcome"`
	Checks        []Check   `json:"checks"`
	Witnesses     []Witness `json:"witnesses,omitempty"`
	Metrics       Metrics   `json:"metrics"`
}

func validHex(value string, length int) bool {
	if len(value) != length {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

// Check names, in the order a run establishes them.
const (
	CheckFreshStore = "fresh_store"
	CheckBootstrap  = "bootstrap"
	CheckHeadTail   = "head_tail"
	CheckHeaderSync = "header_sync"
	CheckDAS        = "das"
	CheckDASRecent  = "das_recent"
	CheckRestart    = "restart"
	CheckCleanup    = "cleanup"
)

// RequiredChecks returns the checks every result must contain, as a new slice.
func RequiredChecks() []string {
	return []string{
		CheckFreshStore, CheckBootstrap, CheckHeadTail, CheckHeaderSync,
		CheckDAS, CheckDASRecent, CheckRestart, CheckCleanup,
	}
}

// WitnessChecks names the checks that carry a witness, in report order.
func WitnessChecks() []string { return []string{CheckDAS, CheckDASRecent, CheckRestart} }

// SoftChecks are observed and reported but never decide the overall outcome.
// Live-head sampling depends on how quickly header gossip reaches a brand-new
// peer identity, which varied from three to more than ten minutes in live
// runs; it is published as a timing, not as a verdict.
func SoftChecks() []string { return []string{CheckDASRecent} }

// IsSoft reports whether the named check is excluded from the overall outcome.
func IsSoft(name string) bool {
	for _, soft := range SoftChecks() {
		if soft == name {
			return true
		}
	}
	return false
}

// Aggregate returns inconclusive for a malformed or incomplete set of checks.
func Aggregate(checks []Check) Outcome {
	priorities := map[Outcome]int{Pass: 0, Inconclusive: 1, Unsupported: 2, Fail: 3}
	if len(checks) != len(RequiredChecks()) {
		return Inconclusive
	}
	expected := make(map[string]bool)
	for _, name := range RequiredChecks() {
		expected[name] = true
	}
	result := Pass
	for _, check := range checks {
		priority, known := priorities[check.Outcome]
		if !expected[check.Name] || !known {
			return Inconclusive
		}
		delete(expected, check.Name)
		if !IsSoft(check.Name) && priority > priorities[result] {
			result = check.Outcome
		}
	}
	return result
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// validateWitness checks that a witness record is well formed.
func (r Result) validateWitness(w Witness) error {
	if w.Epoch < 1 || w.Epoch > 2 || w.EvidenceKind != EvidenceNativeLogs {
		return errors.New("invalid witness identity")
	}
	switch w.Check {
	case CheckDAS:
		if w.Epoch != 1 {
			return errors.New("first witness must belong to the first epoch")
		}
	case CheckDASRecent:
		if w.JobType != JobRecent {
			return errors.New("live-head witness must be a recent job")
		}
	case CheckRestart:
		if w.Epoch != 2 {
			return errors.New("restart witness must belong to the second epoch")
		}
	default:
		return errors.New("unknown witness check")
	}
	switch w.JobType {
	case JobRecent, JobCatchup, JobRetry:
	default:
		return errors.New("unknown witness job type")
	}
	h := w.Header
	if h.Height == 0 || h.Width <= 0 || !validHex(h.Hash, 64) || !validHex(h.Root, 64) ||
		h.ChainID != r.Profile.ChainID ||
		h.Time.IsZero() {
		return errors.New("invalid witnessed header")
	}
	amount := r.Profile.SampleAmount
	if h.Width <= amount/h.Width {
		amount = h.Width * h.Width
	}
	if w.Empty {
		if w.Check != CheckDAS {
			return errors.New("only the first-sample witness may complete an empty block")
		}
		amount = 0
	}
	if amount < 0 || w.SampleCount != amount {
		return errors.New("incomplete sample count")
	}
	if w.DurationSeconds < 0 || !finite(w.DurationSeconds) {
		return errors.New("invalid witness duration")
	}
	if w.StartedAt.IsZero() || w.CompletedAt.Before(w.StartedAt) || w.StartedAt.Before(r.StartedAt) ||
		w.CompletedAt.After(r.FinishedAt) {
		return errors.New("witness outside execution interval")
	}
	return nil
}

// Validate checks the metric values, for readers of records derived from a result.
func (m Metrics) Validate() error { return m.validate() }

func (m Metrics) validate() error {
	for _, v := range []*float64{
		m.StartupSeconds, m.HeaderSyncSeconds, m.FirstSampleSeconds,
		m.RecentSampleSeconds, m.RestartResumeSeconds, m.SyncHeadersPerSecond,
	} {
		if v != nil && (*v < 0 || !finite(*v)) {
			return errors.New("invalid metric")
		}
	}
	if (m.BootstrappersConnected == nil) != (m.BootstrappersTotal == nil) {
		return errors.New("bootstrap connectivity needs both counts")
	}
	if m.BootstrappersTotal != nil &&
		(*m.BootstrappersTotal <= 0 || *m.BootstrappersConnected < 0 || *m.BootstrappersConnected > *m.BootstrappersTotal) {
		return errors.New("invalid bootstrap connectivity")
	}
	return nil
}

func (r Result) Validate() error {
	if _, err := json.Marshal(r); err != nil {
		return errors.New("report evidence is not JSON serializable")
	}
	if err := r.Metrics.validate(); err != nil {
		return err
	}
	if len(r.Witnesses) > 3 {
		return errors.New("too many witnesses")
	}
	byCheck := map[string]Witness{}
	for i, w := range r.Witnesses {
		if err := r.validateWitness(w); err != nil {
			return err
		}
		if _, duplicate := byCheck[w.Check]; duplicate {
			return errors.New("duplicate witness check")
		}
		byCheck[w.Check] = w
		for _, earlier := range r.Witnesses[:i] {
			// Every empty square shares one root; only hashes tell them apart.
			sameRoot := !w.Empty && !earlier.Empty && strings.EqualFold(earlier.Header.Root, w.Header.Root)
			if strings.EqualFold(earlier.Header.Hash, w.Header.Hash) || sameRoot {
				return errors.New("witness reuses prior data")
			}
		}
		// Witnesses are recorded in the order they were established.
		if i > 0 && (w.Epoch < r.Witnesses[i-1].Epoch || !w.CompletedAt.After(r.Witnesses[i-1].CompletedAt)) {
			return errors.New("witness completion must follow prior completion")
		}
	}
	if len(r.Witnesses) > 0 && r.Witnesses[0].Check != CheckDAS {
		return errors.New("first witness must be the first sample")
	}
	if recent, ok := byCheck[CheckDASRecent]; ok && recent.Header.Height <= byCheck[CheckDAS].Header.Height {
		return errors.New("recent witness must sample a newer live head")
	}
	if _, ok := byCheck[CheckDAS]; r.Outcome == Pass && !ok {
		return errors.New("PASS requires the first-sample witness")
	}
	if _, ok := byCheck[CheckRestart]; r.Outcome == Pass && !ok {
		return errors.New("PASS requires the restart witness")
	}

	if r.Outcome != Aggregate(r.Checks) {
		return errors.New("overall outcome contradicts checks")
	}

	if r.StartedAt.IsZero() || r.FinishedAt.IsZero() || r.FinishedAt.Before(r.StartedAt) {
		return errors.New("report is unfinished or has inverted time")
	}
	for _, check := range r.Checks {
		if check.DurationSeconds < 0 || !finite(check.DurationSeconds) {
			return errors.New("invalid check duration")
		}
	}

	if r.SchemaVersion != "2" || strings.TrimSpace(r.RunID) == "" || strings.TrimSpace(r.Profile.Name) == "" ||
		strings.TrimSpace(r.Profile.Network) == "" {
		return errors.New("missing or unsupported report identity")
	}
	if r.Outcome == Pass {
		if r.Profile.ChainID == "" || r.NodeID == "" || r.Profile.Image == "" || r.Profile.TelemetrySchema == "" ||
			!validHex(r.Profile.SourceCommit, 40) ||
			!strings.HasPrefix(r.ImageID, "sha256:") ||
			!validHex(strings.TrimPrefix(r.ImageID, "sha256:"), 64) {
			return errors.New("PASS requires bound node/image/source/config identity")
		}
		if r.Profile.SampleAmount <= 0 || r.Profile.SamplingWindowSeconds <= 0 || r.Profile.StorageWindowSeconds <= 0 {
			return errors.New("PASS requires positive effective sampling configuration")
		}
	}

	if len(r.Checks) != len(RequiredChecks()) {
		return errors.New("incomplete report")
	}
	expected := map[string]bool{}
	for _, name := range RequiredChecks() {
		expected[name] = true
	}
	for _, check := range r.Checks {
		if !expected[check.Name] {
			return errors.New("unknown or duplicate check")
		}
		delete(expected, check.Name)
		switch check.Outcome {
		case Pass, Fail, Unsupported, Inconclusive:
		default:
			return errors.New("unknown check outcome")
		}
	}
	return nil
}
