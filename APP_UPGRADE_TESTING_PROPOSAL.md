# Celestia App Upgrade Testing Proposal

## Overview

This proposal outlines a comprehensive testing strategy for Celestia app upgrades, treating upgrade scenarios as **network topology configurations** rather than isolated test categories. This approach ensures realistic testing of actual upgrade scenarios that operators face in production environments.

## Core Philosophy

### **Network Topology Approach**
Instead of spreading upgrade tests across categories, we treat upgrades as **new network topologies** with mixed-version nodes, ensuring:

- **Realistic testing** of actual upgrade scenarios
- **Comprehensive coverage** of individual and network behavior
- **Practical validation** of what operators experience
- **Clear organization** between API and E2E concerns

### **Why This Approach is Better**

1. **Realistic Testing Environment**: Mixed-version networks mirror actual upgrade scenarios
2. **Comprehensive Coverage**: No gaps between individual node and network behavior
3. **Practical Validation**: Tests backward/forward compatibility and network resilience
4. **Clear Organization**: API tests validate individual nodes, E2E tests validate coordination

## Network Topology Configurations

### **Pre-Upgrade Topology**
```
[V5 Bridge Node] + [V5 Light Node] + [V5 Light Node]
```

### **During Upgrade Topology (Mixed-Version)**
```
[V5 Bridge Node] + [V6 Bridge Node] + [V5 Light Node] + [V6 Light Node]
```

### **Post-Upgrade Topology**
```
[V6 Bridge Node] + [V6 Light Node] + [V6 Light Node]
```

## Test Categories and Implementation

### **API Tests** (< 30s each)
Individual node functionality validation in mixed-version environments.

#### **Individual Node API Tests**
```go
func TestV5BridgeNodeAPIs() {
    // Validate v5 bridge node APIs work correctly
    // - Header operations
    // - Blob operations  
    // - State operations
    // - P2P operations
}

func TestV6BridgeNodeAPIs() {
    // Validate v6 bridge node APIs work correctly
    // - Header operations
    // - Blob operations
    // - State operations
    // - P2P operations
}

func TestV5LightNodeAPIs() {
    // Validate v5 light node APIs work correctly
    // - DAS operations
    // - Header sync
    // - Blob retrieval
    // - P2P operations
}

func TestV6LightNodeAPIs() {
    // Validate v6 light node APIs work correctly
    // - DAS operations
    // - Header sync
    // - Blob retrieval
    // - P2P operations
}
```

#### **Cross-Version Compatibility Tests**
```go
func TestV5NodeHandlesV6Headers() {
    // Test v5 node can validate and process v6 headers
    // - Header validation
    // - Block processing
    // - Error handling for unsupported features
}

func TestV6NodeHandlesV5Headers() {
    // Test v6 node can validate and process v5 headers
    // - Backward compatibility
    // - Header validation
    // - Block processing
}

func TestCrossVersionBlobOperations() {
    // Test blob operations between v5 and v6 nodes
    // - Submit via v5, retrieve via v6
    // - Submit via v6, retrieve via v5
    // - Proof generation and verification
}
```

### **E2E Sanity Tests** (1-2 min each)
Basic network coordination and data flow validation.

#### **Mixed-Version Network Stability**
```go
func TestMixedVersionNetworkBasic() {
    // 1 V5 Bridge + 1 V6 Bridge + 1 V5 Light + 1 V6 Light
    // Verify basic network operations work
    
    // Test 1: Network connectivity
    // - All nodes can connect to each other
    // - P2P discovery works across versions
    
    // Test 2: Cross-version data flow
    // - Submit blob via v5 bridge, retrieve via v6 light
    // - Submit blob via v6 bridge, retrieve via v5 light
    
    // Test 3: Header synchronization
    // - Headers sync correctly between versions
    // - No version-specific header processing issues
}
```

#### **Data Availability Across Versions**
```go
func TestCrossVersionDataAvailability() {
    // Test DAS and data retrieval across versions
    
    // Test 1: V5 to V6 data flow
    // - Submit blob via v5 bridge
    // - Verify v6 light node can retrieve and verify DAS
    
    // Test 2: V6 to V5 data flow
    // - Submit blob via v6 bridge
    // - Verify v5 light node can retrieve and verify DAS
    
    // Test 3: Mixed-version DAS coordination
    // - Verify DAS sampling works across version boundaries
    // - Verify share discovery between versions
}
```

#### **Header Sync Between Versions**
```go
func TestHeaderSyncBetweenVersions() {
    // Test header synchronization in mixed-version networks
    
    // Test 1: Forward sync (v5 → v6)
    // - v5 node produces headers
    // - v6 node syncs and validates headers
    
    // Test 2: Backward sync (v6 → v5)
    // - v6 node produces headers
    // - v5 node syncs and validates headers
    
    // Test 3: Network head coordination
    // - Verify network head is consistent across versions
    // - Verify sync state reporting is accurate
}
```

### **E2E P2P Sanity Tests** (2-4 min each)
Complex network scenarios and transition testing.

#### **Upgrade Transition Scenarios**
```go
func TestUpgradeTransitionFlow() {
    // Simulate complete upgrade transition
    
    // Phase 1: Pre-upgrade (all v5)
    // - Setup network with all v5 nodes
    // - Validate baseline functionality
    
    // Phase 2: Mixed-version (v5 + v6)
    // - Introduce v6 nodes gradually
    // - Monitor network stability
    // - Validate cross-version operations
    
    // Phase 3: Post-upgrade (all v6)
    // - Complete transition to v6
    // - Validate all functionality works
    // - Verify no data loss occurred
}
```

#### **Rollback and Recovery Scenarios**
```go
func TestUpgradeFailureRecovery() {
    // Test network behavior during failed upgrades
    
    // Test 1: Partial upgrade failure
    // - Some nodes upgrade, others fail
    // - Verify network can continue operating
    // - Verify rollback mechanisms work
    
    // Test 2: Complete upgrade failure
    // - All nodes fail to upgrade
    // - Verify network can recover to previous state
    // - Verify no data corruption
    
    // Test 3: Mixed failure scenarios
    // - Different failure modes for different node types
    // - Verify graceful degradation
}
```

### **E2E P2P Complex Tests** (3-5 min each)
Advanced scenarios and edge cases.

#### **Network Resilience During Upgrades**
```go
func TestNetworkResilienceDuringUpgrade() {
    // Test network stability under stress during upgrades
    
    // Test 1: High load during upgrade
    // - Continuous blob submission during upgrade
    // - Verify no operations fail
    // - Verify performance remains acceptable
    
    // Test 2: Node failures during upgrade
    // - Simulate node failures during upgrade
    // - Verify network can handle failures
    // - Verify upgrade can complete despite failures
    
    // Test 3: Network partition during upgrade
    // - Simulate network partitions
    // - Verify network can recover
    // - Verify upgrade consistency across partitions
}
```

#### **Performance Impact Assessment**
```go
func TestUpgradePerformanceImpact() {
    // Measure performance impact of mixed-version networks
    
    // Test 1: Baseline performance
    // - Measure performance with all v5 nodes
    
    // Test 2: Mixed-version performance
    // - Measure performance with v5/v6 mix
    // - Compare against baseline
    
    // Test 3: Post-upgrade performance
    // - Measure performance with all v6 nodes
    // - Verify performance improvements/regressions
}
```

## Framework Implementation

### **Upgrade Network Topology Structure**
```go
type UpgradeNetworkTopology struct {
    V5Nodes []Node
    V6Nodes []Node
    Mixed   bool // true when both versions exist
    Config  UpgradeConfig
}

type UpgradeConfig struct {
    V5BridgeNodes int
    V6BridgeNodes int
    V5LightNodes  int
    V6LightNodes  int
    AppVersion    uint64
    NodeVersion   string
}
```

### **Framework Extensions**
```go
func (f *Framework) SetupUpgradeTopology(ctx context.Context, config UpgradeConfig) *UpgradeNetworkTopology {
    // Setup network with specified version distribution
    topology := &UpgradeNetworkTopology{
        Config: config,
        Mixed:  config.V5BridgeNodes > 0 && config.V6BridgeNodes > 0,
    }
    
    // Create v5 nodes
    for i := 0; i < config.V5BridgeNodes; i++ {
        node := f.createBridgeNode(ctx, version: 5)
        topology.V5Nodes = append(topology.V5Nodes, node)
    }
    
    for i := 0; i < config.V5LightNodes; i++ {
        node := f.createLightNode(ctx, version: 5)
        topology.V5Nodes = append(topology.V5Nodes, node)
    }
    
    // Create v6 nodes
    for i := 0; i < config.V6BridgeNodes; i++ {
        node := f.createBridgeNode(ctx, version: 6)
        topology.V6Nodes = append(topology.V6Nodes, node)
    }
    
    for i := 0; i < config.V6LightNodes; i++ {
        node := f.createLightNode(ctx, version: 6)
        topology.V6Nodes = append(topology.V6Nodes, node)
    }
    
    return topology
}

func (f *Framework) PerformUpgradeTransition(ctx context.Context, topology *UpgradeNetworkTopology) error {
    // Simulate upgrade transition
    // 1. Signal upgrade
    // 2. Wait for upgrade height
    // 3. Verify upgrade completion
    // 4. Validate network stability
}
```

### **Test Suite Structure**
```go
type UpgradeTestSuite struct {
    suite.Suite
    framework *Framework
}

func TestUpgradeTestSuite(t *testing.T) {
    suite.Run(t, &UpgradeTestSuite{})
}

func (s *UpgradeTestSuite) SetupSuite() {
    s.framework = NewFramework()
}

func (s *UpgradeTestSuite) TearDownSuite() {
    s.framework.Cleanup()
}
```

## Test Configuration Examples

### **Basic Mixed-Version Network**
```go
var BasicMixedConfig = UpgradeConfig{
    V5BridgeNodes: 1,
    V6BridgeNodes: 1,
    V5LightNodes:  1,
    V6LightNodes:  1,
    AppVersion:    6,
    NodeVersion:   "latest",
}
```

### **Upgrade Transition Config**
```go
var UpgradeTransitionConfig = UpgradeConfig{
    V5BridgeNodes: 2,
    V6BridgeNodes: 0, // Will be added during test
    V5LightNodes:  3,
    V6LightNodes:  0, // Will be added during test
    AppVersion:    6,
    NodeVersion:   "latest",
}
```

### **Stress Test Config**
```go
var StressTestConfig = UpgradeConfig{
    V5BridgeNodes: 3,
    V6BridgeNodes: 3,
    V5LightNodes:  5,
    V6LightNodes:  5,
    AppVersion:    6,
    NodeVersion:   "latest",
}
```

## Implementation Plan

### **Phase 1: Framework Foundation** (Week 1-2)
- Implement `UpgradeNetworkTopology` structure
- Add version-specific node creation methods
- Implement upgrade transition simulation
- Add cross-version compatibility helpers

### **Phase 2: API Tests** (Week 3-4)
- Implement individual node API tests
- Add cross-version compatibility tests
- Validate header and blob operations
- Test error handling and edge cases

### **Phase 3: E2E Sanity Tests** (Week 5-6)
- Implement mixed-version network tests
- Add cross-version data flow validation
- Test header synchronization
- Validate network stability

### **Phase 4: Complex Scenarios** (Week 7-8)
- Implement upgrade transition tests
- Add rollback and recovery scenarios
- Test network resilience
- Performance impact assessment

## Success Criteria

### **Functional Success**
- ✅ All API tests pass in mixed-version environments
- ✅ E2E tests validate network coordination
- ✅ Cross-version data flow works correctly
- ✅ Header synchronization is reliable
- ✅ No data loss during upgrades

### **Performance Success**
- ⚡ Mixed-version networks maintain acceptable performance
- ⚡ Upgrade transitions complete within time limits
- ⚡ No significant performance degradation
- ⚡ Network stability maintained throughout

### **Operational Success**
- 📝 Clear error messages for version incompatibilities
- 📝 Graceful handling of upgrade failures
- 📝 Comprehensive logging of upgrade events
- 📝 Rollback mechanisms work reliably

## Benefits of This Approach

### **1. Realistic Testing**
- Tests actual upgrade scenarios operators face
- Validates mixed-version network behavior
- Ensures gradual rollout compatibility

### **2. Comprehensive Coverage**
- API tests validate individual node functionality
- E2E tests validate network coordination
- No gaps between individual and network behavior

### **3. Practical Validation**
- Tests backward compatibility (v6 nodes with v5 data)
- Tests forward compatibility (v5 nodes with v6 data)
- Validates network resilience during transitions

### **4. Clear Organization**
- API tests focus on individual node behavior
- E2E tests focus on network coordination
- Clear separation of concerns

## Conclusion

This network topology approach to upgrade testing ensures that Celestia nodes handle app upgrades gracefully, maintain network stability, and provide excellent user experience during version transitions. By treating upgrades as network topology scenarios, we create realistic, comprehensive, and practical tests that validate the actual upgrade experience operators will face in production environments.

The implementation plan provides a structured approach to building this testing infrastructure, with clear phases and success metrics to track progress and ensure quality. This approach complements existing app-level testing and provides confidence in the upgrade process across the entire Celestia ecosystem.
