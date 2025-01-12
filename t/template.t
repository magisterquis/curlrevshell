#!/bin/sh
#
# template.t
# Make sure template things work as expected
# By J. Stuart McMurray
# Created 20241211
# Last Modified 20250112

set -e

. t/shmore.subr

tap_plan 3

# Make sure we're not using html/template anywhere.
GOT="$(find . -type f \
        \! -name '*.swp' \
        \! -path './t/*' \
        -exec grep html/template {} + ||:)"

tap_is "$GOT" "" "Not using html/template" "$0" $LINENO

# Make sure we get a warning if we use -callback-template.
GOT="$(go run . -callback-template ./doesnotexist </dev/null 2>&1)"
tap_like \
        "$GOT" \
        '(?s:-callback-template is.*'\
'going away eventually.*'\
'Use -template instead)' \
        "Deprecation warning for -callback-template" \
        "$0" $LINENO

# Make sure we get a warning if our template doesn't exist.
GOT="$(
        while printf '\r'; do sleep .1; done |
        go run . -template ./doesnotexist 2>&1 |
        perl -nE '/(Missing template file \.\/doesnotexist)/ and say $1 and exit'
)"
tap_is \
        "$GOT" \
        "Missing template file ./doesnotexist" \
        "Warning for missing template file" \
        "$0" $LINENO

# vim: ft=sh
