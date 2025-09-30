#!/bin/bash

# Script to check parity between root and tastora go.mod files
# This ensures that shared dependencies have the same versions

set -e

ROOT_GO_MOD="go.mod"
TASTORA_GO_MOD="nodebuilder/tests/tastora/go.mod"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "🔍 Checking parity between root and tastora go.mod files..."

# Check if files exist
if [[ ! -f "$ROOT_GO_MOD" ]]; then
    echo -e "${RED}❌ Root go.mod file not found: $ROOT_GO_MOD${NC}"
    exit 1
fi

if [[ ! -f "$TASTORA_GO_MOD" ]]; then
    echo -e "${RED}❌ Tastora go.mod file not found: $TASTORA_GO_MOD${NC}"
    exit 1
fi

# Function to extract dependencies from go.mod file
extract_deps() {
    local go_mod_file="$1"
    # Extract require section (both direct and indirect)
    grep -E "^\s+[a-zA-Z0-9._/-]+" "$go_mod_file" | \
    grep -v "// indirect" | \
    sed 's/^\s*//' | \
    sort
}

# Function to extract indirect dependencies
extract_indirect_deps() {
    local go_mod_file="$1"
    # Extract indirect dependencies
    grep -E "^\s+[a-zA-Z0-9._/-]+.*// indirect" "$go_mod_file" | \
    sed 's/^\s*//' | \
    sed 's/\s*\/\/ indirect.*$//' | \
    sort
}

# Function to get version for a dependency
get_version() {
    local go_mod_file="$1"
    local dep="$2"
    grep -E "^\s+$dep\s+" "$go_mod_file" | head -1 | awk '{print $2}'
}

# Extract dependencies from both files
echo "📋 Extracting dependencies..."
ROOT_DEPS=$(extract_deps "$ROOT_GO_MOD")
TASTORA_DEPS=$(extract_deps "$TASTORA_GO_MOD")

# Find common dependencies
COMMON_DEPS=$(comm -12 <(echo "$ROOT_DEPS") <(echo "$TASTORA_DEPS"))

if [[ -z "$COMMON_DEPS" ]]; then
    echo -e "${YELLOW}⚠️  No common direct dependencies found${NC}"
else
    echo "🔗 Found $(echo "$COMMON_DEPS" | wc -l) common direct dependencies"
fi

# Check versions for common dependencies
PARITY_ISSUES=0

echo ""
echo "🔍 Checking version parity for common dependencies..."

while IFS= read -r dep; do
    if [[ -n "$dep" ]]; then
        ROOT_VERSION=$(get_version "$ROOT_GO_MOD" "$dep")
        TASTORA_VERSION=$(get_version "$TASTORA_GO_MOD" "$dep")
        
        if [[ "$ROOT_VERSION" != "$TASTORA_VERSION" ]]; then
            echo -e "${RED}❌ Version mismatch for $dep:${NC}"
            echo -e "   Root:    $ROOT_VERSION"
            echo -e "   Tastora: $TASTORA_VERSION"
            PARITY_ISSUES=$((PARITY_ISSUES + 1))
        else
            echo -e "${GREEN}✅ $dep: $ROOT_VERSION${NC}"
        fi
    fi
done <<< "$COMMON_DEPS"

# Check indirect dependencies for critical ones
echo ""
echo "🔍 Checking critical indirect dependencies..."

# List of critical dependencies that should match
CRITICAL_DEPS=(
    "github.com/cosmos/cosmos-sdk"
    "github.com/celestiaorg/celestia-app/v5"
    "github.com/cometbft/cometbft"
    "github.com/gogo/protobuf"
    "github.com/syndtr/goleveldb"
    "github.com/tendermint/tendermint"
    "github.com/ipfs/boxo"
    "github.com/ipfs/go-datastore"
)

for dep in "${CRITICAL_DEPS[@]}"; do
    ROOT_VERSION=$(get_version "$ROOT_GO_MOD" "$dep")
    TASTORA_VERSION=$(get_version "$TASTORA_GO_MOD" "$dep")
    
    if [[ -n "$ROOT_VERSION" && -n "$TASTORA_VERSION" ]]; then
        if [[ "$ROOT_VERSION" != "$TASTORA_VERSION" ]]; then
            echo -e "${RED}❌ Critical dependency version mismatch for $dep:${NC}"
            echo -e "   Root:    $ROOT_VERSION"
            echo -e "   Tastora: $TASTORA_VERSION"
            PARITY_ISSUES=$((PARITY_ISSUES + 1))
        else
            echo -e "${GREEN}✅ $dep: $ROOT_VERSION${NC}"
        fi
    elif [[ -n "$ROOT_VERSION" || -n "$TASTORA_VERSION" ]]; then
        echo -e "${YELLOW}⚠️  $dep present in one file but not the other${NC}"
        echo -e "   Root:    ${ROOT_VERSION:-"not found"}"
        echo -e "   Tastora: ${TASTORA_VERSION:-"not found"}"
    fi
done

# Check replace directives
echo ""
echo "🔍 Checking replace directives..."

ROOT_REPLACES=$(grep -A 100 "^replace (" "$ROOT_GO_MOD" | grep -E "^\s+[a-zA-Z0-9._/-]+" | sed 's/^\s*//' | sort)
TASTORA_REPLACES=$(grep -A 100 "^replace (" "$TASTORA_GO_MOD" | grep -E "^\s+[a-zA-Z0-9._/-]+" | sed 's/^\s*//' | sort)

# Check for replace directive differences
REPLACE_DIFF=$(comm -3 <(echo "$ROOT_REPLACES") <(echo "$TASTORA_REPLACES"))

if [[ -n "$REPLACE_DIFF" ]]; then
    echo -e "${YELLOW}⚠️  Replace directive differences found:${NC}"
    echo "$REPLACE_DIFF"
    # Don't count this as a parity issue since replace directives might legitimately differ
fi

# Summary
echo ""
echo "📊 Summary:"
if [[ $PARITY_ISSUES -eq 0 ]]; then
    echo -e "${GREEN}✅ All dependency versions are in parity!${NC}"
    exit 0
else
    echo -e "${RED}❌ Found $PARITY_ISSUES version mismatch(es)${NC}"
    echo -e "${YELLOW}💡 Please update the tastora go.mod file to match the root go.mod versions${NC}"
    exit 1
fi
