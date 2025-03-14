#!/bin/ksh
#
# debug.t
# Make sure we don't have debugging things left in
# By J. Stuart McMurray
# Created 20250216
# Last Modified 20250302

#set -euo pipefail
set -eo pipefail

. t/shmore.subr
. t/t.subr

tap_plan 3

# Make sure we're not using MQD
GOT=$(gorun </dev/null 2>&1 | egrep 'MQD DEBUG PACKAGE LOADED$' ||:)
tap_is "$GOT" "" "Not using github.com/magisterquis/mqd" "$0" $LINENO

# Check for stderr, at least at startup
ERRF=$(mktemp -t debug.XXXXXXXXXX)
trap 'rm $ERRF; tap_done_testing' EXIT
gorun \
        -ctrl-i /dev/null \
        -log /dev/null \
        -prompt kittens \
        -serve-files-from /dev/null \
        -template /dev/null \
        2>$ERRF |&
for i in `jot 6`; do read -p; done
exec 9>&p; exec 9>&-
wait
tap_is "$(cat <"$ERRF")" "" "Nothing on stderr at startup" "$0" $LINENO

# Make sure we didn't leave any stray DEBUGs lying about.
GOT="$(egrep -InR '#[[:space:]]*DEBUG' | sort -u ||:)"
tap_is \
        "$GOT" \
        "" \
        "No files with DEBUG comments" \
        "$0" $LINENO

# vim: ft=sh
