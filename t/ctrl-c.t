#!/bin/sh
#
# ctrl-c.t
# Make sure Ctrl+C works
# By J. Stuart McMurray
# Created 20241203
# Last Modified 20241203

set -e

. ./t/shmore.subr

tap_plan 3

WANT='Caught Ctrl+C.  One more in the next second to kill the shell.'
GOT="$(printf '\003test\r\n' | go run . 2>&1 | fgrep -o "$WANT")"
tap_is "$GOT" "$WANT" "First Ctrl+C message" "$0" $LINENO

WANT='Caught second Ctrl+C'
GOT="$(printf '\003\003' | go run . 2>&1 | fgrep -o "$WANT")"
tap_is "$GOT" "$WANT" "Second Ctrl+C message" "$0" $LINENO

printf '\003\003' | go run .
GOT=$?
tap_ok "$GOT" "Exited on double Ctrl+C" "$0" $LINENO

# vim: ft=sh
