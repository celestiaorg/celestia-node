package docker

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/client"

	rpcclient "github.com/celestiaorg/celestia-node/api/rpc/client"
)

// RPC returns an HTTP JSON-RPC client with a read-only token created by exec in
// the container. The token and the exec output are kept out of logs and errors.
// The caller owns the client.
func (s *Session) RPC(ctx context.Context) (*rpcclient.Client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.nativeRPC(ctx, false)
}

// nativeRPC creates a client with a read token, or with a 30s admin token for
// the two metadata reads below, the only callers that ask for admin. The
// caller holds mu until the operation and the client's Close complete.
func (s *Session) nativeRPC(ctx context.Context, metadata bool) (*rpcclient.Client, error) {
	permission, ttl := "read", "1h"
	if metadata {
		permission, ttl = "admin", "30s"
	}
	info, err := s.inspect(ctx)
	if err != nil {
		return nil, err
	}
	if metadata && (!s.started || !info.StartedAt.Equal(s.epoch)) {
		return nil, errors.New("metadata requires observed boot epoch")
	}
	if !info.Running {
		return nil, errors.New("RPC requires running container")
	}
	c, err := s.cli.ContainerInspect(ctx, s.containerID, client.ContainerInspectOptions{})
	if err != nil {
		return nil, err
	}
	if c.Container.NetworkSettings == nil {
		return nil, errors.New("missing RPC binding")
	}
	bindings := c.Container.NetworkSettings.Ports[rpcPort]
	if len(bindings) != 1 || bindings[0].HostIP.String() != "127.0.0.1" || bindings[0].HostPort == "" {
		return nil, errors.New("RPC binding is not exact host loopback")
	}
	execCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	exec, err := s.cli.ExecCreate(
		execCtx,
		s.containerID,
		client.ExecCreateOptions{
			Cmd: []string{
				"/bin/celestia",
				"light",
				"auth",
				permission,
				"--node.store",
				storePath,
				"--p2p.network",
				s.cfg.Profile.Network,
				"--ttl",
				ttl,
			},
			AttachStdout: true,
			AttachStderr: true,
			TTY:          false,
			Privileged:   false,
		},
	)
	if err != nil {
		return nil, errors.New("native authorization exec creation failed")
	}
	attached, err := s.cli.ExecAttach(execCtx, exec.ID, client.ExecAttachOptions{TTY: false})
	if err != nil {
		return nil, errors.New("native authorization exec attach failed")
	}
	defer attached.Close()
	stopClose := context.AfterFunc(execCtx, attached.Close)
	defer stopClose()
	var stdout bytes.Buffer
	_, err = stdcopy.StdCopy(&stdout, io.Discard, io.LimitReader(attached.Reader, 32768))
	if err != nil {
		return nil, errors.New("native authorization stream failed")
	}
	state, err := s.cli.ExecInspect(execCtx, exec.ID, client.ExecInspectOptions{})
	if err != nil || state.Running || state.ExitCode != 0 || state.ContainerID != s.containerID {
		return nil, errors.New("native authorization exec did not complete successfully")
	}
	// The node prints this notice to stdout when a custom network is set. Strip
	// exactly that notice and reject any other extra output.
	notice := "WARNING: Celestia custom network specified. " +
		"Only use this option if the node is freshly created and initialized.\n" +
		"**DO NOT** run a custom network over an already-existing node store!"
	token := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(stdout.String()), notice))
	if token == "" || len(token) > 16384 || strings.ContainsAny(token, "\r\n\t ") {
		return nil, errors.New("native authorization output invalid")
	}
	// Plain HTTP is enough for the canary's reads and keeps no websocket open.
	// The token travels in the Authorization header, never in a URL or command.
	rpc, err := rpcclient.NewClient(ctx, "http://"+net.JoinHostPort("127.0.0.1", bindings[0].HostPort), token)
	if err != nil {
		return nil, errors.New("native RPC client construction failed")
	}
	return rpc, nil
}

// P2PInfo reads the node's peer identity. The node requires admin permission
// for it, so the admin client exists only for this call.
func (s *Session) P2PInfo(ctx context.Context) (peer.AddrInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	c, err := s.nativeRPC(ctx, true)
	if err != nil {
		return peer.AddrInfo{}, errors.New("native P2P info unavailable")
	}
	defer c.Close()
	info, err := c.P2P.Info(ctx)
	if err != nil {
		return peer.AddrInfo{}, errors.New("native P2P info unavailable")
	}
	return info, nil
}

// P2PPeers reads the IDs of connected peers with its own short-lived admin
// client, like P2PInfo.
func (s *Session) P2PPeers(ctx context.Context) ([]peer.ID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	c, err := s.nativeRPC(ctx, true)
	if err != nil {
		return nil, errors.New("native P2P peers unavailable")
	}
	defer c.Close()
	peers, err := c.P2P.Peers(ctx)
	if err != nil {
		return nil, errors.New("native P2P peers unavailable")
	}
	return peers, nil
}
