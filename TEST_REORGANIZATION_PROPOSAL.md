# Test Reorganization Proposal for Tastora

### **Proposed Solution**

This reorganization introduces a **four-tier testing strategy** that provides:

1. **API Tests** (< 30s): Pure contract validation for rapid feedback
2. **E2E Sanity** (1-2 min): Basic functionality flows for core validation  
3. **E2E P2P Sanity** (2-4 min): Network topology and multi-node coordination
4. **E2E P2P Complex** (3-5 min): Advanced scenarios and edge cases

### **Scope**

This reorganization covers all major Celestia node modules:
- **Share Module**: DAS operations, namespace isolation, sampling
- **Header Module**: Sync operations, network head management  
- **Blob Module**: Submission, retrieval, proof generation
- **State Module**: Account management, delegation operations
- **P2P Module**: Peer management, bandwidth monitoring
- **Node Module**: Administrative operations, authentication
- **Blobstream Module**: Data root tuple root operations
- **Core Module**: Bridge node core chain integration
- **Pruner Module**: Pruning coordination and archival operations



## Proposed Test Categories

### 1. **API Tests** (Pure API Compliance)
**Setup**: 1 Bridge Node + 1 Light Node  
**Focus**: API contract validation, RPC compliance, parameter validation  
**Execution Time**: Fast (< 30 seconds per test)

- **TestRPCEndpointCompliance**: Raw curl tests against both BN/LN RPC ports
- **TestShareAPIContract**: Validate Share module API responses match expected schema
  - SharesAvailable, GetShare, GetSamples, GetEDS, GetRow, GetNamespaceData, GetRange
- **TestHeaderAPIContract**: Validate Header module API responses
  - LocalHead, GetByHash, GetByHeight, WaitForHeight, GetRangeByHeight, SyncState, SyncWait, NetworkHead, Subscribe
- **TestBlobAPIContract**: Validate Blob module API responses
  - Submit, Get, GetAll, GetProof, Included, GetCommitmentProof, Subscribe
- **TestStateAPIContract**: Validate State module API responses
  - AccountAddress, Balance, BalanceForAddress, Transfer, SubmitPayForBlob, Delegate, Undelegate, QueryDelegation
- **TestP2PAPIContract**: Validate P2P module API responses
  - Info, Peers, PeerInfo, Connect, ClosePeer, Connectedness, NATStatus, BandwidthStats, ResourceState
- **TestNodeAPIContract**: Validate Node module API responses
  - Info, Ready, LogLevelSet, AuthVerify, AuthNew, AuthNewWithExpiry
- **TestNodeInfoAndHealth**: Node info, readiness checks, health metrics
- **TestBlobstreamAPIContract**: Validate Blobstream module API responses
  - GetDataRootTupleRoot, GetDataCommitment, GetProof

### 2. **E2E - Sanity** (Basic Functionality Flows)
**Setup**: 1 Bridge Node + 1 Light Node  
**Focus**: Core functionality validation, upgrade scenarios  
**Execution Time**: Medium (1-2 minutes per test)

- **TestBasicDASFlow**: Submit blob → verify availability → retrieve data with cross-node coordination
- **TestHeaderSyncSanity**: Verify headers sync correctly between BN/LN
- **TestBasicBlobLifecycle**: Submit → sample → retrieve workflow
- **TestStateOperations**: Account creation, balance queries, basic transfers
- **TestP2PConnectivity**: Basic peer discovery and connection establishment
- **TestBlobstreamBasic**: Basic blobstream data root tuple root queries
- **TestPruningSanity**: Verify pruning behavior on light nodes
- **TestNetworkUpgradeV4ToV5**: Validate upgrade path functionality
- **TestAppUpgradeScenarios**: Test node behavior during app upgrades
- **TestCoreIntegration**: Bridge node core chain integration (if configured)

### 3. **E2E - P2P Sanity** (Network Topology & Discovery)
**Setup**: 1 bridge node and 3 light nodes (could do: 2 Bridge Nodes + 5 Light Nodes (acyclic topology))  
**Focus**: Network health, discovery, basic multi-node operations  
**Execution Time**: Medium-Long (2-4 minutes per test)

- **TestBasicNetworkOperation**: 
  - Health metrics validation (local head, sampling stats)
  - Node discovery and peer management
  - Basic blob retrieval across network
  - Header sync across all nodes
- **TestDiscoveryMechanisms**: P2P discovery, peer routing, DHT operations
- **TestBootstrapping**: Node bootstrap and initial sync
- **TestNetworkHeightConvergence**: All nodes reach desired network height
- **TestMultiNodeStateSync**: State synchronization across multiple nodes
- **TestBandwidthAndResourceManagement**: P2P bandwidth stats and resource limits
- **TestPubSubOperations**: P2P pubsub topic management and message propagation
- **TestArchivalNodeDiscovery**: Full/Bridge nodes advertise archival capabilities
- **TestPruningCoordination**: Light nodes coordinate pruning with full nodes
- **TestMultiNodeHeaderSync**: Cross-node header synchronization with consistency validation
- **TestStaggeredNodeSync**: Nodes joining at different times can catch up properly
- **TestHeaderSubscriptionSync**: Real-time header propagation across nodes
- **TestSyncWaitCoordination**: SyncWait coordination across multiple nodes
- **TestHeaderRangeSync**: Header range retrieval and consistency validation
- **TestMultiNodeDASCoordination**: Multiple light nodes coordinate sampling across network
- **TestDASCatchUpScenarios**: Light nodes joining late can catch up efficiently
- **TestDASPerformanceUnderLoad**: DAS performance with rapid block production
- **TestDASNetworkResilience**: DAS continues working during network issues
- **TestCrossNodeSamplingVerification**: Consistent sampling results across nodes
- **TestCrossNodeBlobAvailability**: Blobs submitted on one node become available on others
- **TestMultiNamespaceBlobIsolation**: Blobs in different namespaces are properly isolated
- **TestConcurrentBlobOperations**: Multiple concurrent blob operations work correctly
- **TestBlobProofConsistency**: Blob inclusion proofs are consistent across nodes
- **TestLightNodeBlobAccess**: Light nodes can access blob data without full block storage

### 4. **E2E - P2P Complex/Routing** (Advanced Network Scenarios)
**Setup**: Variable complex topology based on test needs  
**Focus**: Fault tolerance, edge cases, fraud protection  
**Execution Time**: Long (3-5 minutes per test)

- **TestArchivalRequestRouting**: Advanced routing with archival nodes
- **TestNetworkResilienceWithNodeFailure**: Network continues functioning when nodes fail or restart
- **TestConcurrentMultiNodeOperations**: Multiple nodes handle concurrent operations without conflicts
- **TestComplexStateOperations**: Multi-step state operations, delegation flows
- **TestBlobstreamAdvanced**: Complex blobstream proof generation and verification
- **TestResourceExhaustion**: Test behavior under resource constraints
- **TestNetworkPartitioning**: Network split scenarios and recovery
- **TestAuthenticationAndAuthorization**: Token management and permission validation
- **TestPruningEdgeCases**: Complex pruning scenarios with archival nodes
- **TestCoreChainIntegration**: Advanced core chain integration scenarios

## Implementation Strategy

### Phase 1: Create New Test Structure
Create new test files for each category:

```
nodebuilder/tests/tastora/
├── api_test.go              # Pure API tests
├── e2e_sanity_test.go       # Basic functionality flows  
├── e2e_p2p_sanity_test.go   # Network topology tests
├── e2e_p2p_complex_test.go  # Advanced scenarios
└── share_test.go            # Legacy (to be deprecated)
```

### Phase 2: Migrate Existing Tests
Move current share tests to appropriate categories:

**From `share_test.go` → `e2e_p2p_complex_test.go`:**
- `TestCrossNodeDataAvailability`
- `TestDataPropagationTiming`
- `TestMultiNamespaceDataIsolation`
- `TestLightNodeDataAvailabilitySampling`
- `TestNetworkResilienceWithNodeFailure`
- `TestConcurrentMultiNodeOperations`

**From `das_tests` branch → `e2e_p2p_sanity_test.go`:**
- `TestMultiNodeSamplingCoordination`
- `TestDASCatchUpScenarios`
- `TestDASPerformanceUnderLoad`
- `TestDASNetworkResilience`
- `TestCrossNodeSamplingVerification`

**From `sync_tests` branch → `e2e_p2p_sanity_test.go`:**
- `TestCrossNodeHeaderSync`
- `TestStaggeredNodeSync`
- `TestHeaderSubscriptionSync`
- `TestSyncWaitCoordination`
- `TestHeaderRangeSync`

**From `blob_update` branch → `e2e_p2p_sanity_test.go`:**
- `TestCrossNodeBlobAvailability`
- `TestMultiNamespaceBlobIsolation`
- `TestConcurrentBlobOperations`
- `TestBlobProofConsistency`
- `TestLightNodeBlobAccess`

### Phase 3: Add Missing Test Coverage
Implement new tests to fill gaps:

**API Tests:**
- Pure RPC endpoint validation for all 8 modules (Share, Header, Blob, State, P2P, Node, Blobstream, Core)
- Schema compliance checks with proper error responses
- Parameter validation and edge case handling
- Authentication and permission validation
- Raw HTTP/curl tests for API contract validation

**E2E Sanity:**
- Basic DAS workflow validation
- Upgrade path testing (V4 → V5)
- Core functionality verification across all modules
- State operations (accounts, transfers, delegations)
- P2P connectivity and peer management
- Node health and readiness checks
- Pruning behavior validation
- Blobstream basic operations

**E2E P2P Sanity:**
- Network discovery mechanisms and DHT operations
- Health metric validation across network topology
- Multi-node state synchronization
- Bandwidth and resource management
- PubSub operations and message propagation
- Archival node discovery and capabilities
- Pruning coordination between node types
- Cross-node header synchronization and consistency validation
- Staggered node joining and catch-up scenarios
- Real-time header propagation via subscriptions
- SyncWait coordination across multiple nodes
- Header range retrieval and validation
- Multi-node DAS coordination and sampling consistency
- DAS catch-up scenarios for late-joining nodes
- DAS performance under load conditions
- DAS network resilience during issues
- Cross-node sampling verification and consistency
- Cross-node blob availability and retrieval
- Multi-namespace blob isolation and querying
- Concurrent blob operations and consistency
- Blob proof generation and verification
- Light node blob access capabilities

**E2E P2P Complex:**
- Complex routing scenarios with archival nodes
- Edge case handling and error recovery
- Resource exhaustion and constraint testing
- Network partitioning and recovery scenarios
- Authentication and authorization flows
- Complex state operations and delegation chains
- Advanced blobstream proof generation
- Core chain integration edge cases

## Benefits

### 1. **Clear Separation of Concerns**
- API tests focus purely on contract validation
- E2E tests focus on functionality and integration
- Complex tests focus on advanced scenarios

### 2. **Optimized Resource Usage**
- Simple tests use minimal setup (1 BN + 1 LN)
- Complex tests use full topology only when needed
- Faster feedback for basic functionality

### 3. **Better Debugging**
- Failures clearly indicate which layer has issues
- API failures vs functionality failures vs network issues
- Easier to isolate and fix problems

### 4. **Improved CI/CD**
- Can run different test categories based on changes
- API tests run on every PR
- Complex tests run on main branch or significant changes
- Faster overall test execution

### 5. **Comprehensive Coverage**
- From basic API compliance to advanced network scenarios
- Maintains current comprehensive coverage
- Adds missing test scenarios

## Migration Timeline

### Week 1-2: Phase 1
- Create new test file structure
- Set up basic framework for each category
- Define test interfaces and helpers

### Week 3-4: Phase 2  
- Migrate existing tests to new categories
- Ensure all current functionality is preserved
- Update test documentation

### Week 5-6: Phase 3
- Implement missing API tests
- Add new E2E sanity tests
- Expand P2P complex test coverage

### Week 7: Validation
- Run full test suite to ensure coverage
- Performance validation
- Documentation updates
