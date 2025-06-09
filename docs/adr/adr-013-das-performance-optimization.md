# ADR 013: DAS Performance Optimization Analysis

## Status

Proposed

## Context

Data Availability Sampling (DAS) is a critical component of Celestia's light client functionality, enabling nodes to verify data availability without downloading entire blocks. However, performance analysis using advanced code audit techniques has revealed significant bottlenecks in the current DAS implementation that impact network scalability and user experience.

This ADR presents a comprehensive performance analysis conducted using AI-powered static analysis tools that examined 14,468 code entities across 267 packages, identifying 2,258 anomalies with specific focus on DAS-related performance patterns.

## Performance Analysis Methodology

The analysis utilized:
- **Static code analysis** across celestia-node, celestia-app, celestia-core, and rsmt2d
- **Anomaly detection** using embedding distance analysis with threshold-based scoring
- **Relationship mapping** between code entities to identify performance bottlenecks
- **Source code examination** to validate and contextualize findings

## Critical Performance Bottlenecks Identified

### 1. DAS Coordinator Performance Issues (Critical)

**Location**: `das/coordinator.go:64-101`
**Anomaly Score**: 5.11 (threshold: 3.67) - Highest severity

**Issues**:
- Single-threaded coordination loop blocks all DAS operations
- Inefficient concurrency checking on every iteration
- Worker synchronization overhead via WaitGroup patterns
- Sequential job processing without pipelining

**Impact**: Central bottleneck affecting all DAS sampling operations

### 2. Worker Header Retrieval Bottlenecks (Very High)

**Location**: `das/worker.go:176-203`
**Relationship Confidence**: 0.686-0.540 for GetByHeight operations

**Issues**:
- Sequential header fetching: `w.getter.GetByHeight(ctx, height)`
- Missing batch processing (explicit TODO comment at line 181)
- Individual context timeouts per header sample
- No header request aggregation or prefetching

**Impact**: Multiplied performance penalty across all sampling workers

### 3. EDS Retrieval Strategy Inefficiencies (High)

**Location**: `share/eds/retriever.go:224-264`
**Pattern**: Quadrant request timeout chains

**Issues**:
- Sequential quadrant requests with fixed timeouts
- Goroutine proliferation (one per root) without pooling
- No adaptive timeout based on network conditions
- Inefficient quadrant selection strategy

**Impact**: Significant delays in data reconstruction and availability verification

### 4. Storage Layer Performance Bottlenecks (Medium-High)

**Location**: `store/store.go:111-164`
**Anomaly Pattern**: StorageWindow constant (score: 2.80)

**Issues**:
- Synchronous file I/O operations blocking request processing
- Lock contention on storage stripLock for concurrent operations
- Cache miss penalties without background warming
- No batch operations for multiple EDS writes

**Impact**: I/O bound operations affecting both read and write performance

### 5. ShrEx Protocol Network Inefficiencies (Medium)

**Location**: `share/shwap/p2p/shrex/shrexeds/server.go:93-149`
**Anomaly Count**: 16 in bitswap package, 33 in protocol buffer serialization

**Issues**:
- Per-request file opening without connection pooling
- Synchronous stream handling without async processing
- Fixed buffer sizes regardless of transfer characteristics
- Redundant serialization/deserialization patterns

**Impact**: Network layer inefficiencies affecting peer-to-peer data exchange

## Detailed Performance Impact Analysis

### Coordination Overhead
- **Current**: O(n) coordination checks per job
- **Bottleneck**: Single coordinator managing all workers
- **Effect**: Linear degradation with worker count

### Header Retrieval Efficiency
- **Current**: Sequential fetching with individual timeouts
- **Bottleneck**: No request batching or prefetching
- **Effect**: Network round-trip multiplication

### EDS Reconstruction Performance
- **Current**: Fixed quadrant strategy with sequential requests
- **Bottleneck**: No peer performance optimization
- **Effect**: Worst-case reconstruction times under network stress

### Storage I/O Performance
- **Current**: Synchronous operations with coarse-grained locking
- **Bottleneck**: No async I/O or optimistic caching
- **Effect**: I/O bound performance limiting throughput

## Decision

We propose implementing the following performance optimizations in phases:

### Phase 1: Immediate Optimizations (High Impact, Low Effort)

#### 1.1 Batch Header Retrieval
```go
// Replace: w.getter.GetByHeight(ctx, height)
// With: w.getter.GetByHeightRange(ctx, w.state.from, w.state.to)
```

#### 1.2 Worker Pool Implementation
```go
// Replace individual goroutine spawning with managed worker pool
sc.workerPool.Submit(func() { w.run(ctx, sc.samplingTimeout, sc.resultCh) })
```

#### 1.3 Adaptive Quadrant Timeouts
```go
// Implement dynamic timeout adjustment
timeout := rs.calculateAdaptiveTimeout(networkLatency, quadrantSize)
```

### Phase 2: Medium-term Optimizations (High Impact, Medium Effort)

#### 2.1 Async Coordination Architecture
- Replace blocking coordination loop with channel-based async system
- Implement pipeline processing for job management
- Add backpressure mechanisms for overload protection

#### 2.2 EDS Connection Pooling
- Implement peer connection pooling for EDS retrieval
- Add circuit breaker pattern for unreliable peers
- Smart peer selection based on historical performance

#### 2.3 Storage Layer Optimization
- Background cache warming for frequently accessed EDS
- Batch write operations for multiple EDS files
- Fine-grained locking to reduce contention

### Phase 3: Long-term Architectural Improvements (High Impact, High Effort)

#### 3.1 Event-Driven DAS System
- Replace polling-based coordination with event-driven architecture
- Implement priority queues for different job types (catchup, recent, retry)
- Add intelligent workload balancing

#### 3.2 ML-Enhanced Quadrant Selection
- Implement machine learning-based peer performance prediction
- Dynamic quadrant availability scoring
- Parallel quadrant retrieval with early termination

## Performance Monitoring Requirements

Implement comprehensive monitoring for:

### Core Metrics
- **DAS Coordinator Latency**: Coordination loop iteration time
- **Header Retrieval Rate**: Headers processed per second
- **EDS Reconstruction Time**: Time from first quadrant to full square
- **Storage Cache Hit Rate**: Cache effectiveness measurement
- **Network Peer Performance**: Per-peer success rates and latencies

### SLA Targets
- **DAS Sampling Latency**: < 2 seconds for 95th percentile
- **Header Retrieval Rate**: > 100 headers/second sustained
- **EDS Reconstruction**: < 5 seconds for 128x128 squares
- **Storage Cache Hit Rate**: > 80% for recent blocks

## Expected Performance Improvements

Based on the bottleneck analysis and proposed optimizations:

### Immediate Gains (Phase 1)
- **2-3x improvement** in DAS sampling throughput
- **50-70% reduction** in coordination overhead
- **40-60% improvement** in EDS retrieval times

### Medium-term Gains (Phase 2)
- **3-5x improvement** in overall DAS performance
- **30-50% reduction** in storage I/O latency
- **60-80% improvement** in network utilization efficiency

### Long-term Gains (Phase 3)
- **5-10x improvement** in DAS scalability
- **Near-linear scaling** with network size
- **Predictive performance** optimization

## Implementation Plan

### Phase 1 (Immediate - 4-6 weeks)
1. Implement batch header retrieval in worker package
2. Add worker pool management to coordinator
3. Implement adaptive timeouts for quadrant requests
4. Add comprehensive performance monitoring

### Phase 2 (Medium-term - 8-12 weeks)
1. Redesign coordination loop for async processing
2. Implement connection pooling for EDS retrieval
3. Optimize storage layer with background caching
4. Add intelligent peer selection mechanisms

### Phase 3 (Long-term - 16-24 weeks)
1. Implement event-driven DAS architecture
2. Add ML-based performance prediction
3. Implement advanced workload balancing
4. Complete performance optimization validation

## Consequences

### Positive
- **Significant performance improvements** across all DAS operations
- **Better network scalability** supporting larger block sizes
- **Improved user experience** with reduced latency
- **Enhanced reliability** through better error handling and timeouts
- **Data-driven optimization** using comprehensive monitoring

### Negative
- **Increased implementation complexity** requiring careful testing
- **Memory overhead** from connection pooling and caching
- **Monitoring infrastructure** requirements for performance tracking
- **Migration complexity** for existing deployments

### Risks
- **Regression potential** during optimization implementation
- **Network behavior changes** affecting peer interactions
- **Resource consumption** increases during transition period

## References

- [Celestia DAS Documentation](../das/README.md)
- [Performance Analysis Audit Data](../../data/packages/celestia-node/packages/das/audit_results.json)
- [EDS Implementation Details](../share/eds/README.md)
- [ShrEx Protocol Specification](../share/shwap/p2p/shrex/README.md)

## Validation Criteria

Performance improvements will be validated through:

1. **Benchmark Testing**: Automated performance regression testing
2. **Load Testing**: Network simulation under various conditions
3. **Monitoring Metrics**: Real-time performance tracking in testnet
4. **Peer Feedback**: Community testing and feedback collection

---

**Author**: Performance Analysis Team  
**Date**: January 9, 2025  
**Last Updated**: January 9, 2025