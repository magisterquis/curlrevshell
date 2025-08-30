#!/bin/ksh
#
# tmplfuncs_docs.t
# Make sure tmplfuncs' docs are correct
# By J. Stuart McMurray
# Created 20250613
# Last Modified 20250830

set -euo pipefail

. t/shmore.subr

tap_plan 6

TFDIR=./lib/crstemplate/tmplfuncs
TFDOC="$TFDIR/godoc.txt"
READM="$TFDIR/README.md"
TEMPD="$(mktemp -d)"
GODOC="$TEMPD/godoc.new"
GDRMM="$TEMPD/godoc.readme"
trap 'rm -rf "$TEMPD"; tap_done_testing' EXIT

# Generate new godoc, make sure it doesn't end in a blank line.
go doc -all "$TFDIR" | sed '$d' >"$GODOC"
tap_ok $? "Godoc generated happily" "$0" $LINENO
LASTL=$(tail -n 1 <"$GODOC")
tap_isnt "$LASTL" "" "Godoc does not end in blank line" "$0" $LINENO

# Is the godoc.txt file current?
GOT=$(diff -u "$TFDOC" "$GODOC" ||:)
tap_is "$GOT" "" "$TFDOC correct" "$0" $LINENO

# Is the godoc in the readme correct?
awk '/^```text$/,/^```$/{if ($0 !~ /^```/) {print}}' "$READM" >"$GDRMM"
AWKEC=$?
GOT=$(diff -u "$GDRMM" "$GODOC" ||:)
tap_ok    $AWKEC           "Awk searched $READM for godoc happily" "$0" $LINENO
tap_isnt "$(<"$GDRMM")" "" "Extracted godoc from $READM"           "$0" $LINENO
tap_is   "$GOT"         "" "Godoc in $READM correct"               "$0" $LINENO

# vim: ft=sh
