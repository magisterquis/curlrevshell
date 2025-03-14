#!/bin/ksh
#
# ctrl-i.t
# Tests for inserting things
# By J. Stuart McMurray
# Created 20241204
# Last Modified 20250314

. t/shmore.subr
. t/t.subr

# Temporary logfile
LOGF="$(mktemp -t curlrevshell.ctrl-i.log.XXXXXXXX)"
GOTF="$(mktemp -t curlrevshell.ctrl-i.out.XXXXXXXX)"
trap 'RET=$?; rm -f "$LOGF" "$GOTF"; (exit $RET); tap_done_testing' EXIT

# check_ctrl_i spawns curlrevshell -ctrl-i set to $1,
# connects curl to it, sends it a SIGUSR1 to get the -ctrl-i output, and
# make sure it's $1.want.  Uses $LOGF and $GOTF.
#
# Arguments
# $1 - A Ctrl+I file
function check_ctrl_i {
        set -euo pipefail
        tap_plan 4

        # If we don't have the want file, not much we can do
        HAVEF="$1"
        WANTF="$1.want"
        if ! [[ -f "$WANTF" ]]; then
                tap_fail "Don't have want file $WANTF" "$0" $LINENO
                tap_skip "Missing want file" 3
                return
        fi

        # Reset the logfile
        > "$LOGF"

        # Start curlrevshell going.
        gorun -ctrl-i "$1" -log "$LOGF" |&
        # Skip past the welcome messages.
        while read -p; do if [[ -z "$REPLY" ]]; then break; fi; done

        # Connect up curl to get the Ctrl+I output after a SIGUSR1.
        read -p CURL; 
        CURL=${CURL%/c*}/io
        <&p $CURL -T. --no-progress-meter --output $GOTF 2>&1 &
        while read -p; do
                if [[ "$REPLY" == *"Shell is ready to go"* ]]; then
                        break
                fi
        done

        # Get curlrevshell's pid and send a SIGUSR1.
        CRSPID="$(head -n 1 "$LOGF" | jq .PID)"
        kill -s USR1 $CRSPID
        tap_ok "$?" "Sent SIGUSR1 to $CRSPID" "$0" $LINENO

        # Work out what we expect to get.
        WANTSIZE=$(($(wc -c <$WANTF)))

        # Work out how much we think we sent.
        for i in `jot 2`; do read -p; done
        CRSSIZE=$(($(echo "$REPLY" | cut -f 3 -d ' ')))
        tap_is "$CRSSIZE" "$WANTSIZE" "Reported size correct" "$0" $LINENO

        # Kill curlrevshell and wait for curl to finish writing.
        exec 9>&p; exec 9>&-
        wait

        # Make sure the output is correct.
        GOTSIZE=$(($(wc -c <$GOTF)))
        tap_is "$GOTSIZE" "$CRSSIZE" "Reported and received sizes consistent" \
                "$0" $LINENO
        GOT="$(diff -u "$WANTF" "$GOTF" ||:)"
        tap_is "$GOT" "" "Template output correct" "$0" $LINENO
}

# One test per Ctrl+I file.
FS=$(find ./t/testdata/ctrl-i -name '*.subr')
NFS=$(($(echo "$FS" | egrep -v '^$' | wc -l)))
tap_plan $((1+$NFS))

# Make sure we actually have test files
tap_isnt "$NFS" 0 "Have test files" "$0" $LINENO

# Test ALL the Ctrl+Is!
for FN in $FS; do
        function subtest { check_ctrl_i "$FN"; }
        tap_subtest "Check $FN" subtest "$0" $LINENO
done

# vim: ft=sh
