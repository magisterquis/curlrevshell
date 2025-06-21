#!/bin/ksh
#
# ctrl-i.t
# Tests for inserting things
# By J. Stuart McMurray
# Created 20241204
# Last Modified 20250621

set -euo pipefail

. t/shmore.subr

tap_plan 2

# Temporary cert file.
CERTF=$(mktemp)
trap 'rm "$CERTF"; tap_done_testing' EXIT
if [[ -f "$CERTF" ]]; then rm "$CERTF"; fi

# get_inserted_data grabs lines from Ctrl+Jing the -ctrl-i file $1
#
# $1 - File to insert.
get_inserted_data() {
        # Curlrevshell as a coprocess.
        go run . \
                -ctrl-i "$1" \
                -listen-address 127.0.0.1:0 \
                -no-timestamps \
                -tls-certificate-cache "$CERTF" |&
        # Wait until we're done printing things before hitting Ctrl+J.
        while read -p; do
                if [[ "$REPLY" == *pinnedpubkey* ]]; then
                        break
                fi
        done
        read -p
        # Hit Ctrl+J twice.  This relies on a bit of buffering.
        # Should be fine.
        print -p '\012\012'
        # Get just the inserted bits.
        perl -e '
                # Get to the start of the first inserted block.
                while (<>) {
                        if (/Would have sent the following \d+ bytes/) {
                                print;
                                last;
                        }
                }
                # Print the rest of the block.
                while (<>) {
                        if (/Would have sent the following \d+ bytes/) {
                                last;
                        }
                        print;
                }
        ' <&p
        # Kill curlrevshell.  Waititng takes a bit longer but makes debugging
        # a bit easier.
        exec 3>&p; exec 3>&-; wait
}

# Make sure a ctrl-i file which shouldn't print tab_list doesnt.
GOT=$(get_inserted_data t/testdata/empty.subr)
tap_is \
        "$(($(echo -E "$GOT" | wc -l)))" "5" \
        "Generated non-empty tab_list by default" \
        "$0" $LINENO

GOT=$(get_inserted_data t/testdata/notablist.subr)
tap_unlike \
        "$GOT" 'tab_list\(\) \{' \
        "No tab_list with NOTABLIST" \
        "$0" $LINENO

# vim: ft=sh
