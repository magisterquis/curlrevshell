#!/bin/ksh
#
# tmplfuncs_docs.t
# Make sure tmplfuncs' docs are correct
# By J. Stuart McMurray
# Created 20250613
# Last Modified 20251008

set -euo pipefail

. t/shmore.subr

tap_plan 7

DOCTM=./doc/template.md
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

# Does the table list all of the functions?
cat "$DOCTM" |&
while read -pr; do # Get to the table section
        if [[ "$REPLY" = Functions ]]; then
                break
        fi
done
read -pr; read -pr; read -pr; read -pr # Skip from section header to table
GOT=
while read -pr; do # Get the functions
        if [[ -z "$REPLY" ]]; then
                break
        fi
        GOT="$GOT $(echo "$REPLY" | cut -f 1 -d ' ')"
done
while read -pr; do :; done
wait
WANT=$(go doc ./lib/crstemplate/tmplfuncs |
        perl -ne '/^func ([^\(]+)/&&print " ", lc $1' | sort -u)
tap_is "$GOT" "$WANT" "All template functions listed in $DOCTM" "$0" $LINENO

# vim: ft=sh
