#!/bin/sh
#
# version.t
# Make sure we print a version number
# By J. Stuart McMurray
# Created 20241203
# Last Modified 20241204

set -e

. t/shmore.subr

tap_plan 1

# Unfortunately, we'll just get (devel) here.  Better than nothing?
WANT="Welcome to curlrevshell version (devel)"
GOT="$(echo -n | go run . -no-timestamps 2>&1 | fgrep -o "$WANT")"
tap_is "$GOT" "$WANT" "Version looks ok" "$0" $LINENO

# vim: ft=sh
