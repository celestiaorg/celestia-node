#!/usr/bin/env bash
# Usage: fuzz-only-deadline.sh <go-test-log> <fuzz-target>
#
# Copied from celestia-core's scripts/fuzz-only-deadline.sh.
#
# Exits 0 if the only failure in a `go test -fuzz` log is the fuzz coordinator
# reporting its own -fuzztime deadline. When -fuzztime expires, the
# coordinator can read the expired context before the workers' child context
# is cancelled, so it prints "--- FAIL: <target>" followed only by
# "context deadline exceeded", without writing a failing input. That is a
# clean finish, not a crash: a real crash always writes a failing input or
# prints more than the bare error.
set -euo pipefail

log=$1
target=$2

if grep -q 'Failing input written' "$log"; then
	exit 1
fi
awk -v target="$target" '
	$0 ~ "^--- FAIL: " target " \\(" { failing = 1; fails++; next }
	failing && /^(FAIL|exit status|ok)([[:space:]]|$)/ { failing = 0 }
	failing && NF {
		if ($0 ~ /^[[:space:]]+context deadline exceeded$/) { deadline++ } else { other++ }
	}
	END { exit !(fails == 1 && deadline == 1 && other == 0) }
' "$log"
