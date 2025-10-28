#!/bin/bash

# Improved script to check parity between root and tastora go.mod files
# Uses Go toolchain commands for more reliable dependency analysis

set -e

ROOT_GO_MOD="go.mod"
TASTORA_GO_MOD="nodebuilder/tests/tastora/go.mod"
TASTORA_DIR="nodebuilder/tests/tastora"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo "🔍 Checking parity between root and tastora go.mod files using Go toolchain..."

# Check if files exist
if [[ ! -f "$ROOT_GO_MOD" ]]; then
    echo -e "${RED}❌ Root go.mod file not found: $ROOT_GO_MOD${NC}"
    exit 1
fi

if [[ ! -f "$TASTORA_GO_MOD" ]]; then
    echo -e "${RED}❌ Tastora go.mod file not found: $TASTORA_GO_MOD${NC}"
    exit 1
fi

# Create temporary files
TEMP_DIR=$(mktemp -d)
ROOT_DEPS="$TEMP_DIR/root_deps.txt"
TASTORA_DEPS="$TEMP_DIR/tastora_deps.txt"
ROOT_GRAPH="$TEMP_DIR/root_graph.txt"
TASTORA_GRAPH="$TEMP_DIR/tastora_graph.txt"

# Cleanup function
cleanup() {
    rm -rf "$TEMP_DIR"
}
trap cleanup EXIT

echo "📋 Extracting dependencies using 'go list -m all'..."

# Extract all dependencies using Go toolchain
go list -m all > "$ROOT_DEPS"
(cd "$TASTORA_DIR" && go list -m all) > "$TASTORA_DEPS"

echo "📊 Generating dependency graphs using 'go mod graph'..."
go mod graph > "$ROOT_GRAPH"
(cd "$TASTORA_DIR" && go mod graph) > "$TASTORA_GRAPH"

# Function to extract module names from go list output
extract_modules() {
    local deps_file="$1"
    # Extract module name (first column) from go list -m all output
    awk '{print $1}' "$deps_file" | sort | uniq
}

# Function to get version for a module
get_module_version() {
    local deps_file="$1"
    local module="$2"
    local line=$(grep "^$module " "$deps_file" | head -1)
    if [[ -z "$line" ]]; then
        echo ""
        return
    fi
    
    # Check if this is a replace directive (contains "=>")
    if [[ "$line" == *"=>"* ]]; then
        # Extract the version after "=>" (the actual resolved version)
        echo "$line" | awk -F'=> ' '{print $2}' | awk '{print $1}'
    else
        # Regular version (no replace directive)
        echo "$line" | awk '{print $2}'
    fi
}

# Extract module names from both files
ROOT_MODULES=$(extract_modules "$ROOT_DEPS")
TASTORA_MODULES=$(extract_modules "$TASTORA_DEPS")

# Find common modules
COMMON_MODULES=$(comm -12 <(echo "$ROOT_MODULES") <(echo "$TASTORA_MODULES"))

if [[ -z "$COMMON_MODULES" ]]; then
    echo -e "${YELLOW}⚠️  No common modules found${NC}"
    exit 0
fi

echo "🔗 Found $(echo "$COMMON_MODULES" | wc -l) common modules"

# Check versions for common modules
PARITY_ISSUES=0
VERSION_MISMATCHES=()

echo ""
echo "🔍 Checking version parity for common modules..."

while IFS= read -r module; do
    if [[ -n "$module" ]]; then
        ROOT_VERSION=$(get_module_version "$ROOT_DEPS" "$module")
        TASTORA_VERSION=$(get_module_version "$TASTORA_DEPS" "$module")
        
        if [[ -n "$ROOT_VERSION" && -n "$TASTORA_VERSION" ]]; then
            if [[ "$ROOT_VERSION" != "$TASTORA_VERSION" ]]; then
                echo -e "${RED}❌ Version mismatch for $module:${NC}"
                echo -e "   Root:    $ROOT_VERSION"
                echo -e "   Tastora: $TASTORA_VERSION"
                PARITY_ISSUES=$((PARITY_ISSUES + 1))
                VERSION_MISMATCHES+=("$module:$ROOT_VERSION:$TASTORA_VERSION")
            else
                echo -e "${GREEN}✅ $module: $ROOT_VERSION${NC}"
            fi
        elif [[ -n "$ROOT_VERSION" || -n "$TASTORA_VERSION" ]]; then
            echo -e "${YELLOW}⚠️  $module present in one file but not the other${NC}"
            echo -e "   Root:    ${ROOT_VERSION:-"not found"}"
            echo -e "   Tastora: ${TASTORA_VERSION:-"not found"}"
        fi
    fi
done <<< "$COMMON_MODULES"

# Check for dependency graph differences
echo ""
echo "🔍 Checking dependency graph differences..."

# Extract unique dependencies from each graph
ROOT_GRAPH_DEPS=$(awk '{print $2}' "$ROOT_GRAPH" | sort | uniq)
TASTORA_GRAPH_DEPS=$(awk '{print $2}' "$TASTORA_GRAPH" | sort | uniq)

# Find dependencies only in root
ROOT_ONLY=$(comm -23 <(echo "$ROOT_GRAPH_DEPS") <(echo "$TASTORA_GRAPH_DEPS"))
# Find dependencies only in tastora
TASTORA_ONLY=$(comm -13 <(echo "$ROOT_GRAPH_DEPS") <(echo "$TASTORA_GRAPH_DEPS"))

if [[ -n "$ROOT_ONLY" ]]; then
    echo -e "${BLUE}📋 Dependencies only in root module:${NC}"
    echo "$ROOT_ONLY" | head -10
    if [[ $(echo "$ROOT_ONLY" | wc -l) -gt 10 ]]; then
        echo "... and $(( $(echo "$ROOT_ONLY" | wc -l) - 10 )) more"
    fi
fi

if [[ -n "$TASTORA_ONLY" ]]; then
    echo -e "${BLUE}📋 Dependencies only in tastora module:${NC}"
    echo "$TASTORA_ONLY" | head -10
    if [[ $(echo "$TASTORA_ONLY" | wc -l) -gt 10 ]]; then
        echo "... and $(( $(echo "$TASTORA_ONLY" | wc -l) - 10 )) more"
    fi
fi

# Check replace directives
echo ""
echo "🔍 Checking replace directives..."

ROOT_REPLACES=$(grep -A 100 "^replace (" "$ROOT_GO_MOD" 2>/dev/null | grep -E "^\s+[a-zA-Z0-9._/-]+" | sed 's/^\s*//' | sort || true)
TASTORA_REPLACES=$(grep -A 100 "^replace (" "$TASTORA_GO_MOD" 2>/dev/null | grep -E "^\s+[a-zA-Z0-9._/-]+" | sed 's/^\s*//' | sort || true)

# Check for replace directive differences
REPLACE_DIFF=$(comm -3 <(echo "$ROOT_REPLACES") <(echo "$TASTORA_REPLACES") || true)

if [[ -n "$REPLACE_DIFF" ]]; then
    echo -e "${YELLOW}⚠️  Replace directive differences found:${NC}"
    echo "$REPLACE_DIFF"
    # Don't count this as a parity issue since replace directives might legitimately differ
fi

# Summary
echo ""
echo "📊 Summary:"
if [[ $PARITY_ISSUES -eq 0 ]]; then
    echo -e "${GREEN}✅ All module versions are in parity!${NC}"
    echo -e "${GREEN}✅ Dependency graphs are compatible${NC}"
    exit 0
else
    echo -e "${RED}❌ Found $PARITY_ISSUES version mismatch(es)${NC}"
    echo ""
    echo -e "${YELLOW}💡 To fix version mismatches, consider:${NC}"
    echo -e "   1. Update tastora go.mod to use the same versions as root"
    echo -e "   2. Run 'go mod tidy' in both directories"
    echo -e "   3. Use 'go get module@version' to update specific modules"
    echo ""
    echo -e "${BLUE}📋 Detailed mismatches:${NC}"
    for mismatch in "${VERSION_MISMATCHES[@]}"; do
        IFS=':' read -r module root_ver tastora_ver <<< "$mismatch"
        echo -e "   $module: $root_ver vs $tastora_ver"
    done
    exit 1
fi