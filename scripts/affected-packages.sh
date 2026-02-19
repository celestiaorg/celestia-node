#!/usr/bin/env bash
# Prints the set of Go packages affected by changes relative to a base ref.
# Falls back to all packages if dependencies changed or no Go files changed.
#
# Usage: ./scripts/affected-packages.sh <base-ref> [exclude-pattern]
# Example: ./scripts/affected-packages.sh origin/main nodebuilder/tests

set -euo pipefail

BASE_REF=${1:-origin/main}
EXCLUDE=${2:-nodebuilder/tests}

ALL_PKGS=$(go list ./... | grep -v "$EXCLUDE")

# If go.mod/go.sum changed, dependency graph may have changed — run everything
if git diff --name-only "${BASE_REF}...HEAD" | grep -qE '^go\.(mod|sum)$'; then
  echo "go.mod/go.sum changed: running all packages" >&2
  echo "$ALL_PKGS"
  exit 0
fi

# Get changed .go files
CHANGED_GO=$(git diff --name-only "${BASE_REF}...HEAD" -- '*.go')

if [ -z "$CHANGED_GO" ]; then
  echo "No Go files changed: running all packages" >&2
  echo "$ALL_PKGS"
  exit 0
fi

# Map changed files to packages
CHANGED_PKGS=$(echo "$CHANGED_GO" \
  | sed 's|/[^/]*\.go$||' \
  | sort -u \
  | while read -r dir; do
      go list "./$dir" 2>/dev/null || true
    done \
  | sort -u)

if [ -z "$CHANGED_PKGS" ]; then
  echo "No Go packages resolved: running all packages" >&2
  echo "$ALL_PKGS"
  exit 0
fi

# Find transitive reverse dependencies using go list import graph
# Uses Python for graph traversal (available on ubuntu-latest and macos-14)
echo "$ALL_PKGS" \
  | xargs go list -f '{{.ImportPath}} {{join .Imports " "}}' \
  | python3 - <<PYEOF
import sys, collections

rdeps = collections.defaultdict(set)
all_pkgs = []
for line in sys.stdin:
    parts = line.strip().split()
    if not parts:
        continue
    pkg = parts[0]
    all_pkgs.append(pkg)
    for imp in parts[1:]:
        rdeps[imp].add(pkg)

changed = set("""$CHANGED_PKGS""".strip().split())

affected = set(changed)
queue = list(changed)
while queue:
    pkg = queue.pop()
    for rdep in rdeps.get(pkg, []):
        if rdep not in affected:
            affected.add(rdep)
            queue.append(rdep)

for pkg in all_pkgs:
    if pkg in affected:
        print(pkg)
PYEOF
