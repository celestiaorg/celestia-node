#!/usr/bin/env bash
# Usage: scripts/fuzz-targets.sh list|check|run [FUZZTIME]
#
# The fuzz targets CI runs are the matrix entries in .github/workflows/fuzz.yml,
# one per line in the form `- {package: <dir>, target: <FuzzXxx>, priority: <n>}`.
#
#   list   print "<package> <target> <priority>" for every matrix entry.
#   check  fail unless every `func Fuzz*` in the module is in the matrix or in
#          EXCLUDED below, and every matrix entry names an existing target.
#   run    fuzz every matrix entry for FUZZTIME (default 30s), stopping at the
#          first failure. This is `make fuzz`.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

workflow=.github/workflows/fuzz.yml

# Targets deliberately left out of the workflow, as "<package> <target>|<reason>".
EXCLUDED=(
	"store/file Fuzz_writeReadheader|round-trips the node's own on-disk file header, which no peer supplies"
)

matrix() {
	sed -nE 's/^[[:space:]]*-[[:space:]]*\{package:[[:space:]]*([^,]+),[[:space:]]*target:[[:space:]]*([^,]+),[[:space:]]*priority:[[:space:]]*([0-9]+)\}.*/\1 \2 \3/p' "$workflow"
}

# Prints "<package> <target>" for every fuzz target in the root module. The
# Tastora tests are a separate module and are not fuzzed.
declared() {
	git ls-files --cached --others --exclude-standard -- '*_test.go' ':!nodebuilder/tests/tastora/**' |
		while read -r file; do
			sed -nE 's/^func (Fuzz[A-Za-z0-9_]*)\(.*/\1/p' "$file" |
				while read -r target; do
					echo "$(dirname "$file") $target"
				done
		done | sort -u
}

case "${1:-}" in
list)
	matrix
	;;
check)
	status=0
	in_matrix=$(matrix | awk '{print $1" "$2}' | sort)
	excluded=$(printf '%s\n' "${EXCLUDED[@]}" | cut -d'|' -f1 | sort)
	all=$(declared)
	if [[ -z $in_matrix ]]; then
		echo "error: no matrix entries found in $workflow" >&2
		exit 1
	fi
	while read -r entry; do
		[[ -z $entry ]] && continue
		if ! grep -qxF "$entry" <<<"$in_matrix" && ! grep -qxF "$entry" <<<"$excluded"; then
			echo "error: $entry is neither in the $workflow matrix nor excluded in $0" >&2
			status=1
		fi
		if grep -qxF "$entry" <<<"$in_matrix" && grep -qxF "$entry" <<<"$excluded"; then
			echo "error: $entry is both in the $workflow matrix and excluded in $0" >&2
			status=1
		fi
	done <<<"$all"
	while read -r entry; do
		[[ -z $entry ]] && continue
		if ! grep -qxF "$entry" <<<"$all"; then
			echo "error: $entry is listed but no such fuzz target exists" >&2
			status=1
		fi
	done < <(printf '%s\n%s\n' "$in_matrix" "$excluded")
	dupes=$(awk '{print $1" "$2}' <<<"$in_matrix" | uniq -d)
	if [[ -n $dupes ]]; then
		echo "error: duplicate matrix entries: $dupes" >&2
		status=1
	fi
	if [[ $status == 0 ]]; then
		echo "fuzz targets: $(wc -l <<<"$in_matrix") in the matrix, ${#EXCLUDED[@]} excluded"
	fi
	exit $status
	;;
run)
	fuzztime=${2:-30s}
	log=$(mktemp)
	trap 'rm -f "$log"' EXIT
	matrix | while read -r pkg target _; do
		echo "--> $target in ./$pkg for $fuzztime"
		if ! go test -run '^$' -fuzz "^${target}\$" -fuzztime "$fuzztime" -fuzzminimizetime 10s "./$pkg" 2>&1 | tee "$log"; then
			# go test can report its own -fuzztime deadline as a failure.
			scripts/fuzz-only-deadline.sh "$log" "$target" || exit 1
			echo "--> $target: only the -fuzztime deadline was reported; no failing input"
		fi
	done
	;;
*)
	echo "usage: $0 list|check|run [FUZZTIME]" >&2
	exit 2
	;;
esac
