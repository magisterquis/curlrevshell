#!/bin/ksh
#
# ctrl-i.t
# Tests for inserting things
# By J. Stuart McMurray
# Created 20241204
# Last Modified 20251011

set -euo pipefail

. t/shmore.subr
. t/t.subr

# One test per Ctrl+I file.
FS=$(find ./t/testdata/ctrl-i -maxdepth 1 -name '*.subr')
NFS=$(($(echo "$FS" | egrep -v '^$' | wc -l)))
tap_plan $((4+$NFS))

# Make sure we actually have test files
tap_isnt "$NFS" 0 "Have test files" "$0" $LINENO

# Temporary files
LOGF="$(mktemp -t curlrevshell.ctrl-i.log.XXXXXXXX)"
GOTF="$(mktemp -t curlrevshell.ctrl-i.out.XXXXXXXX)"
CRTF=$(mktemp)
if [[ -f "$CRTF" ]]; then rm "$CRTF"; fi
trap 'rm -f "$LOGF" "$GOTF" "$CRTF"; tap_done_testing' EXIT

# start_curlrevshell starts curlrevshell in a coprocess with the -ctrl-i file
# $1.
#
# Arguments:
# $1 - File to insert.
start_curlrevshell() {
        # Curlrevshell as a coprocess.
        local _ctrl_i_file=$1
        ( go run -ldflags \
                '-X github.com/magisterquis/curlrevshell/lib/opshell.TestingTTY=yes' \
                . \
                -ctrl-i "$_ctrl_i_file" \
                -listen-address 127.0.0.1:0 \
                -no-timestamps \
                -prompt '' \
                -tls-certificate-cache "$CRTF" |
        perl -pE '$|=1;s/\r\n$/\n/' ) |&
}

# read_until reads from the coprocess until a line matching $1 is found.
#
# Arguments:
# $1 - A glob pattern to match
read_until() {
        while read -pr; do
                if [[ "$REPLY" == $1 ]]; then
                        break
                fi
        done
}

# close_curlrevshell closes the coprocess's stdin and waits for it to exit.
close_curlrevshell() { exec 3>&p; exec 3>&-; wait; }

# get_inserted_data grabs lines from Ctrl+Jing the -ctrl-i file $1
#
# Arguments:
# $1 - File to insert.
get_inserted_data() {
        local _ctrl_i_file=$1
        local _got

        # Start curlrevshell going.
        start_curlrevshell "$_ctrl_i_file"

        # Work out what we would have inserted.
        print -p '\023'
        read_until 'Would have sent the following*'
        COUNT=${REPLY#Would have sent the following }
        COUNT=${COUNT% bytes:}
        _got=$(dd bs=1 count=$((COUNT)) status=none <&p)

        # Close curlrevshell before we return.
        close_curlrevshell

        # Return what would have been inserted.
        echo -E "$_got"
}

# Make sure an nonexistent file gives us a warning.
subtest() {
        FN=t/testdata/nonexistent
        tap_plan 3
        start_curlrevshell "$FN"
        # Should get a warning by default.
        read_until "Warning: Ctrl+I file $FN does not exist (yet)"
        tap_ok "$?" "Warned that nonexistent file doesn't exist" "$0" $LINENO
        # Get past the curl line
        read_until 'curl -sk --pinnedpubkey sha256//*'
        tap_ok $? "Got to end of welcome messages" "$0" $LINENO
        # See what we would have gotten
        print -p '\023'
        WANT='Error working out what what would have been inserted: preparing t/testdata/nonexistent: converting t/testdata/nonexistent: unable to get file info: stat t/testdata/nonexistent: no such file or directory'
        read_until "$WANT"
        tap_ok "$?" "Got warning after Ctrl+S" "$0" $LINENO
        close_curlrevshell
}
tap_subtest "Nonexistent file" subtest "$0" $LINENO


# Make sure a ctrl-i file which shouldn't print tab_list doesnt.
GOT=$(get_inserted_data t/testdata/ctrl-i/empty.subr)
WANT="tab_list() {
        echo 'tab_list  - This function list'
}"
tap_is \
        "$GOT" "$WANT" \
        "Generated non-empty tab_list by default" \
        "$0" $LINENO
GOT=$(get_inserted_data t/testdata/ctrl-i/notablist.subr)
WANT='# TABDOC:NOTABLIST'
tap_is \
        "$GOT" "$WANT" \
        "No tab_list with NOTABLIST" \
        "$0" $LINENO

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

# Test ALL the Ctrl+Is!
for FN in $FS; do
        function subtest { check_ctrl_i "$FN"; }
        tap_subtest "Check $FN" subtest "$0" $LINENO
done

# vim: ft=sh
