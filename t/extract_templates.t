#!/bin/ksh
#
# extract_templates.t
# Make sure extract_templates.awk works
# By J. Stuart McMurray
# Created 20250219
# Last Modified 20260808

set -euo pipefail

. t/shmore.subr

tap_plan 4

TMPLD=$(mktemp -td test.tmpl.d.XXXXXXXXXX)
trap 'rm -r "$TMPLD"; tap_done_testing' EXIT

# Extract ALL the (test) templates
./t/extract_templates.awk -v TMPLD="$TMPLD" t/testdata/extract_templates.md
tap_ok $? "Awk exited with status 0" "$0" $LINENO

# Make sure we actually got templates
tap_isnt "$(ls "$TMPLD")" "" "List extracted template files" "$0" $LINENO

# Make sure they're what one expects
set +e
GOT=$(diff -u "$TMPLD" t/testdata/extract_templates.md.want)
RET=$?
set -e
tap_is "$RET" 0  "Diff ran happily"            "$0" $LINENO
tap_is "$GOT" "" "Extracted templates correct" "$0" $LINENO

# vim: ft=sh
