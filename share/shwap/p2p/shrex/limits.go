package shrex

// This file defines the resource manager limits for the shrex server.
//
// # Two-tier enforcement model
//
// Limits are applied at two scopes in the libp2p resource manager:
//
//  1. Protocol scope — enforced at stream creation, before the handler runs.
//     These limits are per request type and provide cheap early rejection.
//     Only stream counts are tracked here; memory is not yet reserved.
//
//  2. Service scope — enforced inside the handler via SetService + ReserveMemory.
//     These limits cover the shrex service as a whole and track both stream
//     counts and memory. Memory is tracked here (not at protocol scope) because
//     ReserveMemory is called inside the handler after SetService; tracking it
//     at both scopes would double-count.
//
// # How limits scale with hardware
//
// Global and per-protocol totals use rcmgr's BaseLimitIncrease so they grow
// with available memory. The autoscale formula is:
//
//	actual = base + (increase × mebibytesAvailable) >> 10
//
// where mebibytesAvailable = totalRAM / 8 / 1 MiB. On an 8 GiB machine,
// mebibytesAvailable = 1024, so actual = base + increase. The "increase" value
// therefore represents how many additional streams (or bytes) are granted per
// 1 GiB of available memory (= 1/8 of total RAM).
//
// Per-peer limits use a fixed BaseLimit with no increase, so they remain
// constant regardless of hardware.

import (
	"os"

	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

// serviceName is the libp2p resource manager service name for the shrex server.
const serviceName = "shrex"

// disableResourceLimits disables shrex resource manager limits when set to "1".
// Intended for debugging and testing; do not use in production.
var disableResourceLimits = os.Getenv("CELESTIA_SHREX_DISABLE_RESOURCE_LIMITS") == "1"

// serviceBaseStreams is the baseline service-wide stream count for the shrex
// server on low-memory machines. It is additive with streamIncrease, which
// scales with available RAM. 128 streams gives a reasonable floor for nodes
// that happen to be memory-constrained.
const serviceBaseStreams = 128

// autoscaleMemUnit is the RAM quantum that maps to one unit of streamIncrease.
// It equals the rcmgr "1 GiB of available memory" unit (1/8 of total RAM on
// typical machines). Expressing streamIncrease as (1 GiB / maxResponseSize)
// ensures the node can hold exactly one additional response per unit of
// available RAM, keeping stream and memory limits self-consistent.
const autoscaleMemUnit = 1024 * 1024 * 1024 // 1 GiB in bytes

// minSimultaneousPeers is the minimum number of peers the server should be
// able to serve at full per-peer stream concurrency at the same time. It drives
// the service-level per-peer stream cap:
//
//	servicePeerStreams = streamIncrease / minSimultaneousPeers
//
// A higher value gives each individual peer fewer streams but allows more peers
// to be served in parallel. 4 is a conservative floor that prevents a single
// peer from monopolising the service stream budget.
const minSimultaneousPeers = 4

// peerStreamsPerProtocol is the maximum number of concurrent inbound streams a
// single peer may open per shrex request type. These limits fire at stream
// creation (protocol scope) and are based on realistic client concurrency:
//
//   - SampleID: a light node runs 16 DAS workers that may each request 16
//     samples simultaneously, so up to 256 parallel streams target a single
//     bridge node in the worst case.
//   - NamespaceDataID / RangeNamespaceDataID / RowID: clients issue at most a
//     handful of parallel queries per block; 16 is a generous ceiling.
//   - EdsID: one request per block-sync cycle; 8 provides ample headroom.
//
// These per-protocol per-peer values are intentionally higher than the
// service-level per-peer cap (servicePeerStreams), which is capacity-derived to
// ensure fairness across multiple simultaneous peers. The two limits serve
// different roles: protocol limits enforce workload-driven ceilings per request
// type; the service limit enforces overall fairness.
var peerStreamsPerProtocol = map[string]int{
	(&shwap.SampleID{}).Name():             256,
	(&shwap.NamespaceDataID{}).Name():      16,
	(&shwap.RangeNamespaceDataID{}).Name(): 16,
	(&shwap.RowID{}).Name():                16,
	(&shwap.EdsID{}).Name():                8,
}

// SetResourceLimits registers shrex resource limits into cfg for both the
// shrex service and each per-protocol handler.
//
// Service limits (AddServiceLimit / AddServicePeerLimit):
//   - Global totals scale with RAM via BaseLimitIncrease.
//   - Per-peer cap is fixed and capacity-derived: streamIncrease / minSimultaneousPeers.
//     On a machine where maxResponseSize = 32 MiB this gives streamIncrease = 32,
//     so servicePeerStreams = 8. Each peer can open up to 8 concurrent streams
//     across all request types, guaranteeing at least 4 peers can be fully served
//     in parallel before the service stream budget is exhausted.
//   - Memory is tracked at this scope because ReserveMemory is called inside the
//     handler after SetService; the budget is maxResponseSize per stream.
//
// Protocol limits (AddProtocolLimit / AddProtocolPeerLimit):
//   - Global totals mirror the service limits so the ceilings are consistent.
//   - Per-peer caps come from peerStreamsPerProtocol and reflect per-type client
//     concurrency (e.g. 256 for SampleID to accommodate a full DAS round).
//   - No memory is tracked here; that is handled exclusively at the service scope.
func SetResourceLimits(cfg *rcmgr.ScalingLimitConfig, networkID string) {
	if disableResourceLimits {
		log.Warn("server: resource limits disabled via CELESTIA_SHREX_DISABLE_RESOURCE_LIMITS")
		return
	}
	// worst-case response size across all registered request types sets the
	// per-stream memory budget and drives the stream increase value.
	// ResponseSize expects the EDS size (full square width after erasure coding),
	// which is 2× the ODS size. share.MaxSquareSize is the ODS upper bound.
	maxEDSSize := share.MaxSquareSize * 2
	var maxMem int64
	for _, newReq := range registry {
		if m := int64(newReq().ResponseSize(maxEDSSize)); m > maxMem {
			maxMem = m
		}
	}

	// streamIncrease = how many additional streams fit in one autoscaleMemUnit.
	// e.g. maxMem = 32 MiB → streamIncrease = 1 GiB / 32 MiB = 32.
	streamIncrease := int(autoscaleMemUnit / maxMem)
	baseMemory := int64(serviceBaseStreams) * maxMem
	increaseMemory := int64(streamIncrease) * maxMem

	// servicePeerStreams is the per-peer service-level stream cap. Derived from
	// capacity so that minSimultaneousPeers peers can all be fully served at once.
	servicePeerStreams := streamIncrease / minSimultaneousPeers
	servicePeerMem := int64(servicePeerStreams) * maxMem

	globalBase := rcmgr.BaseLimit{
		Streams:        serviceBaseStreams,
		StreamsInbound: serviceBaseStreams,
		Memory:         baseMemory,
	}
	globalIncrease := rcmgr.BaseLimitIncrease{
		Streams:        streamIncrease,
		StreamsInbound: streamIncrease,
		Memory:         increaseMemory,
	}
	peerBase := rcmgr.BaseLimit{
		Streams:        servicePeerStreams,
		StreamsInbound: servicePeerStreams,
		Memory:         servicePeerMem,
	}

	cfg.AddServiceLimit(serviceName, globalBase, globalIncrease)
	cfg.AddServicePeerLimit(serviceName, peerBase, rcmgr.BaseLimitIncrease{})

	// Protocol limits enforce stream counts only. Memory is omitted because
	// ReserveMemory is called inside the handler (after SetService), so all
	// memory accounting happens at the service scope. Adding it here too would
	// double-count and could cause spurious rejections.
	for _, newReq := range registry {
		req := newReq()
		protoID := ProtocolID(networkID, req.Name())
		n := peerStreamsPerProtocol[req.Name()]
		perPeer := rcmgr.BaseLimit{
			Streams:        n,
			StreamsInbound: n,
		}
		protoBase := rcmgr.BaseLimit{
			Streams:        serviceBaseStreams,
			StreamsInbound: serviceBaseStreams,
		}
		protoIncrease := rcmgr.BaseLimitIncrease{
			Streams:        streamIncrease,
			StreamsInbound: streamIncrease,
		}
		cfg.AddProtocolLimit(protoID, protoBase, protoIncrease)
		cfg.AddProtocolPeerLimit(protoID, perPeer, rcmgr.BaseLimitIncrease{})
	}
}
