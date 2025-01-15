#!/bin/sh
#
# usage.t
# Make sure the usage statement in the main README is correct
# By J. Stuart McMurray
# Created 20250115
# Last Modified 20250115

set -e

. t/shmore.subr

tap_plan 1

# Stick data in temporary files and delete them when we're done.
WANT=$(mktemp -t current.XXXXXXXXXX)
GOT=$( mktemp -t  readme.XXXXXXXXXX)
trap 'rm $WANT $GOT; tap_done_testing' EXIT

# Grab the usage statement from the code and README
go run . -h >$WANT 2>&1
awk '
        /^Usage$/ { ok = 1; next }
        /^-+$/ { next }
        /^```$/ { if (ok) { nbt++; if (2 == nbt) {exit 0} }; next }
        {if (ok) {print} }
' README.md >$GOT

# Make sure they're the same.
DIFF=$(diff -u $GOT $WANT || true)
tap_is "$DIFF" "" "Usage statement correct" "$0" $LINENO

# vim: ft=sh
