#!/bin/ksh
#
# extract_templates.t
# Make sure extract_templates.awk works
# By J. Stuart McMurray
# Created 20250219
# Last Modified 20250302

set -e

. t/shmore.subr

TMPLD=$(mktemp -td test.tmpl.d.XXXXXXXXXX)
trap 'rm -r "$TMPLD"; tap_done_testing' EXIT

# Extract ALL the (test) templates
./t/extract_templates.awk -v TMPLD="$TMPLD" t/testdata/extract_templates.md
tap_ok $? "Awk exited with status 0" "$0" $LINENO # -e means $? will be 0

# Make sure we actually got templates
tap_isnt "$(ls "$TMPLD")" "" "List extracted template files" "$0" $LINENO

# Make sure they're what one expects
tap_is \
        "$(diff -u "$TMPLD" t/testdata/extract_templates.md.want)" \
        "" \
        "Extracted templates correct"
tap_ok $? "Extracted files correct" "$0" $LINENO # -e means $? will be 0

# vim: ft=sh
