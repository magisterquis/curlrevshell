#!/bin/sh
#
# usage.t
# Make sure the usage statement in the main README is correct
# By J. Stuart McMurray
# Created 20250115
# Last Modified 20250216

. t/shmore.subr
. t/t.subr

tap_plan 1

# Stick data in temporary files and delete them when we're done.
WANTF=$(mktemp -t current.XXXXXXXXXX)
GOTF=$( mktemp -t  readme.XXXXXXXXXX)
trap 'rm -f $WANTF $GOTF; tap_done_testing' EXIT

# Grab the usage statement from the code and README
gorun -h >$WANTF 2>&1
awk '
        /^Usage$/ { ok = 1; next }
        /^-+$/ { next }
        /^```$/ { if (ok) { nbt++; if (2 == nbt) {exit 0} }; next }
        {if (ok) {print} }
' README.md >$GOTF

# Make sure they're the same.
DIFF=$(diff -u $GOTF $WANTF || true)
tap_is "$DIFF" "" "Usage statement correct" "$0" $LINENO

# vim: ft=sh
