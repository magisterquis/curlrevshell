#!/bin/ksh
#
# debug.t
# Make sure we don't have debugging things left in
# By J. Stuart McMurray
# Created 20250216
# Last Modified 20251011

set -euo pipefail

. t/shmore.subr
. t/t.subr

tap_plan 4

# Make sure we're not using MQD
GOT=$(gorun </dev/null 2>&1 | egrep 'MQD DEBUG PACKAGE LOADED$' ||:)
tap_is "$GOT" "" "Not using github.com/magisterquis/mqd" "$0" $LINENO


# Check for stderr, at least at startup
ERRF=$(mktemp)
trap 'rm "$ERRF"; tap_done_testing' EXIT
set +e
GOT=$(gorun \
        -ctrl-i           /dev/null \
        -log              /dev/null \
        -prompt           kittens   \
        -serve-files-from /dev/null \
        -template         /dev/null \
        </dev/null 2>$ERRF)
RET=$?
GOT=$(echo -E "$GOT" | grep -i error ||:)
ERR=$(<$ERRF)
set -e
tap_is "$RET" 0  "Exited happily"                 "$0" $LINENO
tap_is "$GOT" "" "No errors on stdout at startup" "$0" $LINENO
tap_is "$ERR" "" "Nothing on stderr at startup"   "$0" $LINENO

# vim: ft=sh
