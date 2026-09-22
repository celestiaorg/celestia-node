// Package docker runs one fresh light node in Docker and cleans up after it.
package docker

import (
	"context"
	"crypto/rand"
	"errors"
	"net/netip"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"

	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/model"
)

const (
	runLabel   = "org.celestia.canary.run"
	ownerLabel = "org.celestia.canary.owner"
	storePath  = "/home/celestia/canary"
)

var rpcPort = network.MustParsePort("26658/tcp")

type Config struct {
	Profile                  model.Profile
	RunID                    string
	ExistingNetworkID        string
	AdditionalStartArguments []string
}

// Session touches only the resources labeled with its random ownership nonce.
// Its methods run one at a time; callers own the RPC clients and log streams
// they return.
type Session struct {
	mu                                                sync.Mutex
	cli                                               *client.Client
	cfg                                               Config
	owner                                             string
	volumeID, networkID, containerID, imageID         string
	networkOwned, volumeAttempted, containerAttempted bool
	closed                                            bool
	epoch                                             time.Time
	started, graceful                                 bool
	logs                                              *attachStream
	logsUsed                                          bool
}

func (s *Session) labels() map[string]string {
	return map[string]string{runLabel: s.cfg.RunID, ownerLabel: s.owner}
}

func (s *Session) owns(labels map[string]string) bool {
	return labels[runLabel] == s.cfg.RunID && labels[ownerLabel] == s.owner
}

func New(ctx context.Context, cfg Config) (out *Session, retErr error) {
	if !regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,48}$`).MatchString(cfg.RunID) {
		return nil, errors.New("invalid run ID")
	}
	localImage := regexp.MustCompile(`^sha256:[a-f0-9]{64}$`).MatchString(cfg.Profile.Image)
	if !localImage && !regexp.MustCompile(`^[^\s@]+@sha256:[a-f0-9]{64}$`).MatchString(cfg.Profile.Image) {
		return nil, errors.New("image must be an immutable repository@sha256 digest or exact local sha256 ID")
	}
	if cfg.Profile.Network == "" {
		return nil, errors.New("profile network required")
	}
	if (cfg.ExistingNetworkID == "" || cfg.Profile.Name != model.ProfileLocal) &&
		(len(cfg.AdditionalStartArguments) > 0 || cfg.Profile.CustomNetwork != "") {
		return nil, errors.New("public network overrides forbidden")
	}
	for _, arg := range cfg.AdditionalStartArguments {
		key, value, ok := strings.Cut(arg, "=")
		if !ok || value == "" ||
			!slices.Contains([]string{"--headers.trusted-peers", "--p2p.mutual", "--core.ip", "--core.port"}, key) {
			return nil, errors.New("unsafe local start argument")
		}
	}
	cfg.AdditionalStartArguments = slices.Clone(cfg.AdditionalStartArguments)
	cfg.Profile.Bootstrappers = slices.Clone(cfg.Profile.Bootstrappers)
	cli, err := client.New(client.FromEnv)
	if err != nil {
		return nil, err
	}
	s := &Session{cli: cli, cfg: cfg, owner: rand.Text(), volumeID: "cfn-" + cfg.RunID + "-data"}
	defer func() {
		if retErr != nil {
			retErr = errors.Join(retErr, s.Cleanup(context.Background()))
		}
	}()
	_, err = cli.VolumeInspect(ctx, s.volumeID, client.VolumeInspectOptions{})
	if err == nil {
		return nil, errors.New("fresh volume already exists")
	}
	if !errdefs.IsNotFound(err) {
		return nil, err
	}
	_, err = cli.ContainerInspect(ctx, "cfn-"+cfg.RunID+"-node", client.ContainerInspectOptions{})
	if err == nil {
		return nil, errors.New("container name already exists")
	}
	if !errdefs.IsNotFound(err) {
		return nil, err
	}
	if !localImage {
		pull, err := cli.ImagePull(ctx, cfg.Profile.Image, client.ImagePullOptions{})
		if err != nil {
			return nil, err
		}
		err = pull.Wait(ctx)
		pull.Close()
		if err != nil {
			return nil, err
		}
	}
	image, err := cli.ImageInspect(ctx, cfg.Profile.Image)
	if err != nil {
		return nil, err
	}
	s.imageID = image.ID
	if !regexp.MustCompile(`^sha256:[a-f0-9]{64}$`).MatchString(s.imageID) ||
		(localImage && s.imageID != cfg.Profile.Image) ||
		(!localImage && !slices.Contains(image.RepoDigests, cfg.Profile.Image)) {
		return nil, errors.New("image digest identity mismatch")
	}
	if cfg.Profile.SourceCommit == "" || image.Config == nil ||
		image.Config.Labels["org.opencontainers.image.revision"] != cfg.Profile.SourceCommit {
		return nil, errors.New("image source identity mismatch")
	}
	s.networkID = cfg.ExistingNetworkID
	if s.networkID == "" {
		netName := "cfn-" + cfg.RunID + "-net"
		_, err = cli.NetworkInspect(ctx, netName, client.NetworkInspectOptions{})
		if err == nil {
			return nil, errors.New("network name already exists")
		}
		if !errdefs.IsNotFound(err) {
			return nil, err
		}
		s.networkID = netName
		s.networkOwned = true
		created, err := cli.NetworkCreate(
			ctx,
			netName,
			client.NetworkCreateOptions{Driver: "bridge", Labels: s.labels()},
		)
		if err != nil {
			return nil, err
		}
		s.networkID = created.ID
	}
	ni, err := cli.NetworkInspect(ctx, s.networkID, client.NetworkInspectOptions{})
	if err != nil {
		return nil, err
	}
	if ni.Network.Driver != "bridge" {
		return nil, errors.New("session requires a local bridge network")
	}
	if s.networkOwned && !s.owns(ni.Network.Labels) {
		return nil, errors.New("network ownership mismatch")
	}
	s.networkID = ni.Network.ID
	s.volumeAttempted = true
	v, err := cli.VolumeCreate(ctx, client.VolumeCreateOptions{Name: s.volumeID, Driver: "local", Labels: s.labels()})
	if err != nil {
		return nil, err
	}
	if v.Volume.Name != s.volumeID || !s.owns(v.Volume.Labels) {
		return nil, errors.New("fresh volume ownership collision")
	}
	// init prints private key material, so both of its output streams are
	// discarded before they reach Docker logs. exec makes celestia PID 1, so it
	// receives SIGTERM directly instead of through a shell.
	script := `set -eu
network="$1"; shift
if [ ! -f /home/celestia/canary/config.toml ]; then
 /bin/celestia light init --node.store /home/celestia/canary --p2p.network "$network" >/dev/null 2>&1 \
  || { printf 'native initialization failed\n' >&2; exit 1; }
fi
exec /bin/celestia light start --node.store /home/celestia/canary --p2p.network "$network" "$@"`
	// The node's JSON stderr is the evidence stream: das and share/light log
	// their sampling records at debug level.
	args := make([]string, 0, 8+len(cfg.AdditionalStartArguments))
	args = append(args,
		script,
		"canary",
		cfg.Profile.Network,
		"--rpc.addr=0.0.0.0",
		"--rpc.port=26658",
		"--rpc.skip-auth=false",
		"--log.level=INFO",
		"--log.level.module=das:debug,share/light:debug",
	)
	args = append(args, cfg.AdditionalStartArguments...)
	env := []string{"GOLOG_OUTPUT=stderr", "GOLOG_LOG_FMT=json"}
	if cfg.Profile.CustomNetwork != "" {
		env = append(env, "CELESTIA_CUSTOM="+cfg.Profile.CustomNetwork)
	}
	s.containerID = "cfn-" + cfg.RunID + "-node"
	s.containerAttempted = true
	created, err := cli.ContainerCreate(ctx, client.ContainerCreateOptions{
		Name: s.containerID,
		Config: &container.Config{
			Image:        s.imageID,
			Entrypoint:   []string{"/bin/sh", "-c"},
			Cmd:          args,
			Env:          env,
			Labels:       s.labels(),
			Tty:          false,
			StopSignal:   "SIGTERM",
			ExposedPorts: network.PortSet{rpcPort: {}},
		},
		HostConfig: &container.HostConfig{
			NetworkMode:   container.NetworkMode(s.networkID),
			PortBindings:  network.PortMap{rpcPort: {{HostIP: netip.MustParseAddr("127.0.0.1"), HostPort: "0"}}},
			Mounts:        []mount.Mount{{Type: mount.TypeVolume, Source: s.volumeID, Target: "/home/celestia"}},
			RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyDisabled},
			LogConfig:     container.LogConfig{Type: "json-file"},
		},
	})
	if err != nil {
		return nil, err
	}
	s.containerID = created.ID
	return s, nil
}

func (s *Session) Store() model.StoreIdentity {
	s.mu.Lock()
	defer s.mu.Unlock()
	return model.StoreIdentity{VolumeID: s.volumeID, Owner: s.cfg.RunID, Fresh: s.volumeAttempted}
}

// Cleanup keeps the caller's deadline (30s if there is none) but ignores its
// cancellation, and that budget includes waiting for other session operations.
// It removes only this session's resources, leaves a borrowed network in place
// and reads back each removal. If the wait times out, nothing is removed and
// Cleanup can be retried.
func (s *Session) Cleanup(ctx context.Context) error {
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(30 * time.Second)
	}
	cleanup, cancel := context.WithDeadline(context.WithoutCancel(ctx), deadline)
	defer cancel()
	// Poll the operation mutex rather than block on it in a goroutine that could
	// wait forever behind a graceful Stop.
	wait := time.NewTicker(5 * time.Millisecond)
	defer wait.Stop()
	for {
		if err := cleanup.Err(); err != nil {
			return err
		}
		if s.mu.TryLock() {
			break
		}
		select {
		case <-cleanup.Done():
			return cleanup.Err()
		case <-wait.C:
		}
	}
	defer s.mu.Unlock()
	if err := cleanup.Err(); err != nil {
		return err
	}
	if s.closed {
		return nil
	}
	var errs []error
	if s.logs != nil {
		s.logs.Close()
	}
	foreignContainer := false
	if s.containerAttempted {
		c, err := s.cli.ContainerInspect(cleanup, s.containerID, client.ContainerInspectOptions{})
		switch {
		case errdefs.IsNotFound(err):
			s.containerAttempted = false
		case err != nil:
			errs = append(errs, err)
		case c.Container.Config == nil || !s.owns(c.Container.Config.Labels):
			// A failed create may resolve the attempted name to a foreign
			// container. Keep it, but reconcile our own volume/network with
			// non-force removal (Docker still rejects resources in use).
			// Unknown config or drift of a known container ID stays unresolved.
			foreignContainer = c.Container.Config != nil && s.containerID == "cfn-"+s.cfg.RunID+"-node"
			errs = append(errs, errors.New("refuse cleanup: container ownership mismatch"))
		case c.Container.ID == "" || (s.containerID != "cfn-"+s.cfg.RunID+"-node" && c.Container.ID != s.containerID):
			errs = append(errs, errors.New("refuse cleanup: container identity mismatch"))
		default:
			// Resolve an ambiguous create once, then delete/read back that
			// exact owned ID rather than a name another container can reuse.
			s.containerID = c.Container.ID
			_, err = s.cli.ContainerRemove(cleanup, s.containerID, client.ContainerRemoveOptions{Force: true})
			if err == nil || errdefs.IsNotFound(err) {
				_, err = s.cli.ContainerInspect(cleanup, s.containerID, client.ContainerInspectOptions{})
				if errdefs.IsNotFound(err) {
					s.containerAttempted = false
					err = nil
				} else if err == nil {
					err = errors.New("container survived cleanup")
				}
			}
			if err != nil {
				errs = append(errs, err)
			}
		}
	}
	if s.volumeAttempted && (!s.containerAttempted || foreignContainer) {
		v, err := s.cli.VolumeInspect(cleanup, s.volumeID, client.VolumeInspectOptions{})
		switch {
		case errdefs.IsNotFound(err):
			s.volumeAttempted = false
		case err != nil:
			errs = append(errs, err)
		case !s.owns(v.Volume.Labels):
			errs = append(errs, errors.New("refuse cleanup: volume ownership mismatch"))
		default:
			_, err = s.cli.VolumeRemove(cleanup, s.volumeID, client.VolumeRemoveOptions{})
			if err == nil || errdefs.IsNotFound(err) {
				_, err = s.cli.VolumeInspect(cleanup, s.volumeID, client.VolumeInspectOptions{})
				if errdefs.IsNotFound(err) {
					s.volumeAttempted = false
					err = nil
				} else if err == nil {
					err = errors.New("volume survived cleanup")
				}
			}
			if err != nil {
				errs = append(errs, err)
			}
		}
	}
	if s.networkOwned && (!s.containerAttempted || foreignContainer) {
		n, err := s.cli.NetworkInspect(cleanup, s.networkID, client.NetworkInspectOptions{})
		switch {
		case errdefs.IsNotFound(err):
			s.networkOwned = false
		case err != nil:
			errs = append(errs, err)
		case !s.owns(n.Network.Labels):
			errs = append(errs, errors.New("refuse cleanup: network ownership mismatch"))
		default:
			_, err = s.cli.NetworkRemove(cleanup, s.networkID, client.NetworkRemoveOptions{})
			if err == nil || errdefs.IsNotFound(err) {
				_, err = s.cli.NetworkInspect(cleanup, s.networkID, client.NetworkInspectOptions{})
				if errdefs.IsNotFound(err) {
					s.networkOwned = false
					err = nil
				} else if err == nil {
					err = errors.New("network survived cleanup")
				}
			}
			if err != nil {
				errs = append(errs, err)
			}
		}
	}
	if len(errs) == 0 {
		s.closed = true
		return s.cli.Close()
	}
	return errors.Join(errs...)
}
