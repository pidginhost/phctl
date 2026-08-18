#!/usr/bin/env sh
# Print total statement coverage (as a bare percentage) from a Go coverage
# profile.
#
# Why not `go tool cover -func=... | tail -1`? That command reports *named
# functions only*. phctl puts almost all of its logic in `RunE:` closures, and
# a closure is invisible to it: cmd/compute has 18 named functions and 56 RunE
# closures, and -func lists exactly the 18 -- mostly init() and table helpers
# that tests hit at 100%. It therefore reported 78.4% for a tree whose real
# statement coverage was 34.9%, which is how a 40% gate stayed green while
# sitting below its own threshold.
#
# The profile itself has no such blind spot: every block is in there. Sum it.
#
# Usage: coverage-total.sh <coverage.out>

set -e

PROFILE="${1:-coverage.out}"

if [ ! -f "$PROFILE" ]; then
  echo "ERROR: coverage profile not found: $PROFILE" >&2
  exit 1
fi

awk '
  NR == 1 && /^mode:/ { next }
  NF >= 3 {
    loc = $1
    stmts = $(NF - 1)
    count = $NF
    # A block compiled into more than one test binary appears more than once.
    # Count its statements once, and treat it as covered if any binary hit it.
    total[loc] = stmts
    if (count > 0) hit[loc] = 1
  }
  END {
    for (loc in total) {
      n += total[loc]
      if (loc in hit) c += total[loc]
    }
    if (n == 0) {
      print "ERROR: profile contains no statements" > "/dev/stderr"
      exit 1
    }
    printf "%.1f\n", 100 * c / n
  }
' "$PROFILE"
