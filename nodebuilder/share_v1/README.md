# Share V1 Module - Optimized Share Interface

This module provides an enhanced version of the share module that uses optimized proof structures while maintaining backwards compatibility with the existing share module through a comprehensive conversion layer.

## Overview

The `share_v1` module implements an optimized API with key improvements:

- **Optimized Proof Structure**: Uses `shwap.RangeNamespaceData` with efficient dataroot proofs instead of legacy `types.ShareProof`
- **Backwards Compatibility**: Provides seamless conversion utilities between old and new proof structures
- **Multiple Interface Options**: Choose between legacy compatibility, full optimization, or hybrid approaches
- **Clean API Design**: Organized, non-redundant interface with clear naming conventions

## API Structure

### 🔄 Legacy Interface (Perfect Backwards Compatibility)
```go
GetRangeWithLegacyProof(ctx, height, start, end) -> (*share.GetRangeResult, error)
```
- ✅ Drop-in replacement for legacy `GetRange`
- ✅ No namespace parameter needed
- ❌ No optimization benefits

### ⚡ Optimized Interface (New Format)
```go
OptimizedGetRange(ctx, namespace, height, fromCoords, toCoords, proofsOnly) -> (shwap.RangeNamespaceData, error)
```
- ✅ Full optimization benefits
- ✅ New efficient proof format
- ⚠️ Requires namespace parameter

### 🎯 Hybrid Interface (Optimized Backend + Legacy Format)
```go
OptimizedGetRangeWithLegacyFormat(ctx, namespace, height, start, end) -> (*share.GetRangeResult, error)
```
- ✅ Optimization benefits internally
- ✅ Legacy format output
- ⚠️ Requires namespace parameter

## Usage Examples

### Using the Conversion Adapter

```go
import (
    "github.com/celestiaorg/celestia-node/nodebuilder/share"
    "github.com/celestiaorg/celestia-node/nodebuilder/share_v1"
)

// Create modules
shareModule := // ... existing share module
shareV1Module := // ... new share_v1 module

// Create conversion adapter
adapter := share_v1.NewConversionAdapter(shareModule, shareV1Module)

// Option 1: Perfect backwards compatibility (no namespace needed)
legacyResult, err := adapter.GetRangeWithLegacyProof(ctx, height, start, end)

// Option 2: Full optimization with new format
optimizedResult, err := adapter.OptimizedGetRange(ctx, namespace, height, fromCoords, toCoords, false)

// Option 3: Optimization benefits with legacy format (best of both worlds)
hybridResult, err := adapter.OptimizedGetRangeWithLegacyFormat(ctx, namespace, height, start, end)
```

### Direct Usage

```go
// Using the new optimized module directly
shareV1 := // ... new share_v1 module

// Get optimized range data
rangeData, err := shareV1.GetRange(ctx, namespace, height, fromCoords, toCoords, false)
```

## Migration Strategy

1. **Phase 1**: Deploy both modules alongside each other
2. **Phase 2**: Use conversion adapter for gradual migration
3. **Phase 3**: Migrate clients to use optimized interface
4. **Phase 4**: Eventually deprecate legacy module (if desired)

## Implementation Details

- **Core Infrastructure**: Reuses existing share module infrastructure (storage, networking, etc.)
- **Proof Optimization**: Implements efficient dataroot proof verification
- **Memory Efficiency**: Optimized for better memory usage and performance
- **Network Compatibility**: Maintains compatibility with existing network protocols

## Configuration

The share_v1 module reuses the base share configuration:

```go
cfg := share_v1.DefaultConfig(nodeType)
```

## Testing

All existing share module tests should pass when using the backwards compatibility layer, ensuring no regression in functionality.

## Future Considerations

- The conversion utilities provide a bridge during the transition period
- The optimized proof structure is designed for long-term use
- Legacy proof support can be maintained as long as needed for backwards compatibility 