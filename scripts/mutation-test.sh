#!/usr/bin/env bash
# Usage: scripts/mutation-test.sh <package-dir> [file.go ...]
#
# Adapted from celestia-core's scripts/mutation-test.sh.
#
# Runs gremlins mutation testing on one package and writes its JSON report.
# With file arguments, only those files (relative to the package directory)
# are mutated; the package's whole test suite still runs against each mutant.
# The scores are informational: no threshold is set, so a non-zero exit means
# gremlins itself failed (setup, build or test-run error), not a low score.
#
#   GREMLINS             gremlins command (default: gremlins on PATH)
#   MUTATION_OUT         JSON report path (default build/mutation/<pkg>.json)
#   TIMEOUT_COEFFICIENT  multiplier on the measured test time before a mutant
#                        counts as timed out (default 6)
#   WORKERS              parallel mutants (default: gremlins' default, the CPUs)
set -euo pipefail

pkg=${1:?usage: $0 <package-dir> [file.go ...]}
pkg=${pkg#./}
pkg=${pkg%/}
shift

if [[ ! -d $pkg ]]; then
	echo "error: no package directory $pkg" >&2
	exit 2
fi

read -ra gremlins <<<"${GREMLINS:-gremlins}"
out=${MUTATION_OUT:-build/mutation/${pkg//\//_}.json}
mkdir -p "$(dirname "$out")"

args=(unleash --timeout-coefficient "${TIMEOUT_COEFFICIENT:-6}" --output "$out")
if [[ -n ${WORKERS:-} ]]; then
	args+=(--workers "$WORKERS")
fi

# gremlins walks subdirectories too; keep it to the one package. Exclude
# patterns are regexps matched against paths relative to the package.
args+=(--exclude-files '/')

# gremlins has no include filter, so exclude every other non-test file.
if [[ $# -gt 0 ]]; then
	for f in "$@"; do
		if [[ ! -f $pkg/$f ]]; then
			echo "error: no file $pkg/$f" >&2
			exit 2
		fi
	done
	for path in "$pkg"/*.go; do
		name=${path##*/}
		[[ $name == *_test.go ]] && continue
		keep=0
		for f in "$@"; do
			[[ $name == "$f" ]] && keep=1
		done
		if [[ $keep == 0 ]]; then
			args+=(--exclude-files "^${name//./\\.}\$")
		fi
	done
fi

# gremlins times each mutant against the duration of an initial coverage run.
# A cached `go test` result makes that run look instant and every mutant then
# times out, so disable the test cache.
export GOFLAGS="${GOFLAGS:+$GOFLAGS }-count=1"

"${gremlins[@]}" "${args[@]}" "./$pkg"
echo "Report written to $out"
