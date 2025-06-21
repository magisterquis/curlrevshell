#!/bin/ksh
#
# ctrl-c.t
# Make sure Ctrl+C works
# By J. Stuart McMurray
# Created 20241203
# Last Modified 20250621

set -euo pipefail

. ./t/shmore.subr

tap_plan 3

# Temporary cert file.
CERTF=$(mktemp)
trap 'rm "$CERTF"; tap_done_testing' EXIT
if [[ -f "$CERTF" ]]; then rm "$CERTF"; fi

# Do we get a warning after one Ctrl+C?
WANT='Caught Ctrl+C.  One more in the next second to kill the shell.'
GOT=$(
        printf '\003test\r\n' |
        go run . \
                -listen-address 127.0.0.1:0 \
                -tls-certificate-cache "$CERTF" |
        fgrep -o "$WANT"
)
tap_is "$GOT" "$WANT" "First Ctrl+C message" "$0" $LINENO

# Do we get a message after two Ctrl+C's?
WANT='Caught second Ctrl+C'
GOT=$(
        printf '\003\003' |
        go run . \
                -listen-address 127.0.0.1:0 \
                -tls-certificate-cache "$CERTF" |
        fgrep -o "$WANT"
)
tap_is "$GOT" "$WANT" "Second Ctrl+C message" "$0" $LINENO

# Do two Ctrl+C's actually terminate the program?
printf '\003\003' |
go run . \
        -listen-address 127.0.0.1:0 \
        -tls-certificate-cache "$CERTF" >/dev/null
GOT=$?
tap_ok "$GOT" "Exited on double Ctrl+C" "$0" $LINENO

# vim: ft=sh
