#!/bin/ksh
#
# tmplfuncs_docs.t
# Make sure tmplfuncs' docs are correct
# By J. Stuart McMurray
# Created 20250613
# Last Modified 20250613

set -euo pipefail

. t/shmore.subr

TAP_PLAN=2

TFDIR=./lib/crstemplate/tmplfuncs
TFDOC="$TFDIR/godoc.txt"
READM="$TFDIR/README.md"
GODOC=$(go doc -all "$TFDIR")

# Is the godoc.txt file current?
GOT=$(<"$TFDOC")
WANT=$GODOC
tap_is "$GOT" "$WANT" "$TFDOC correct" "$0" $LINENO

# Is the godoc in the readme correct?
GOT=$(awk '/^```text$/,/^```$/{if ($0 !~ /^```/) {print}}' $READM)
WANT=$GODOC
tap_is "$GOT" "$WANT" "Godoc in $READM correct" "$0" $LINENO

# vim: ft=sh
