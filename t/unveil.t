#!/bin/ksh
#
# unveil.t
# Make sure we unveil as expected
# By J. Stuart McMurray
# Created 20251010
# Last Modified 20251026

set -euo pipefail

. t/shmore.subr

tap_plan 10


# get_run_errors gets the errors and exit code from running with /dev/null
# hooked up to stdin.
# On return, GOT will hold lines mentioning errors and RET will hold the exit
# status.
#
# Arguments
# $1 - -log
# $2 - -tls-certificate-cache
get_run_errors() {
        local _log=$1 _cache=$2
        # Run it.
        set +e
        GOT=$(go run . \
                -ctrl-i                /dev/null     \
                -log                   "$_log"       \
                -serve-files-from      /dev/null     \
                -template              /dev/null     \
                -tls-certificate-cache "$_cache"     \
                -listen-address        "127.0.0.1:0" \
                </dev/null 2>&1)
        RET=$?
        # Extract errors
        set -e
        GOT=$(echo -E "$GOT" | grep -i error ||:)
}

# Make sure we can make a certificate and logfile.
TMPD=$(mktemp -d)
TMPF="$TMPD/does/not/exist"
LOGF="$TMPD/a/log/file"
trap 'rm -rf "$TMPD"; tap_done_testing' EXIT
# Can we make a cert cache?
get_run_errors "$LOGF" "$TMPF"
tap_is "$RET" 0  "Nonexistent cert cache - happy exit" "$0" $LINENO
tap_is "$GOT" "" "Nonexistent cert cache - no errors"  "$0" $LINENO
# Were files created?
TCC1=$(<$TMPF) # Was the cert cached?
LOG1=$(<$LOGF) # Was the logfile created?
tap_isnt "$TCC1" "" "Cached certificate created and populated" "$0" $LINENO
tap_isnt "$LOG1" "" "Logfile created and populated"            "$0" $LINENO
# Can we reuse the cache and logfile?
get_run_errors "$LOGF" "$TMPF"
tap_is "$RET" 0  "Existing cert cache - happy exit" "$0" $LINENO
tap_is "$GOT" "" "Existing cert cache - no errors"  "$0" $LINENO
# Were the files reused?
TCC2=$(<$TMPF) # Was the cert cached?
LOG2=$(<$LOGF) # Was the logfile created?
tap_is   "$TCC2" "$TCC1" "Second run - certificate cache unchanged" "$0" $LINENO
tap_isnt "$LOG2" "$LOG1" "Second run - logfile updated"             "$0" $LINENO

# Can we run with no "real" files?
get_run_errors /dev/null /dev/null
tap_is "$RET" 0  "Exited happily with /dev/nulls"    "$0" $LINENO
tap_is "$GOT" "" "No error reported with /dev/nulls" "$0" $LINENO



# vim: ft=sh
