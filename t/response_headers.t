#!/bin/ksh
#
# respnose_headers.t
# Make sure we can set response headers
# By J. Stuart McMurray
# Created 20260627
# Last Modified 20260627

. ./t/t.subr
. ./t/shmore.subr

tap_plan 2

# Check the header aganist WANT.
subtest() {
        gorun |&
        # Get the server pubkey and URL.
        while read -pr; do
                if [[ "To get a shell:" = "$REPLY" ]]; then
                        break
                fi
        done
        read -pr 
        read -pr 
        CURLARGS=$(print -r "$REPLY" | awk '{print $2, $3, $4, $5}')
        tap_like \
                "$CURLARGS" \
                '^-sk --pinnedpubkey sha256//[a-zA-Z0-9+/]{43}= https://127.0.0.1:\d+/c$' \
                "Curl arguments look ok" \
                "$0" $LINENO
        GOT=$(curl \
                --dump-header - \
                --out-null \
                $CURLARGS \
                | tr -d '\r' \
                | egrep -v '^$' \
                | egrep -v '^(Content-Length|Date): ' \
                | sort -u)
        tap_is "$GOT" "$WANT" "Headers correct" "$0" $LINENO
        exec 3>&p; exec 3>&-
        wait
        tap_pass "All child processes exited"
}

# Does it work if we set no headers?
WANT='Content-Type: text/plain; charset=utf-8
HTTP/1.1 200 OK'
tap_subtest "Response headers unset" subtest "$0" $LINENO

# Does it work if we set headers from a file?
TF=t/testdata/response_headers/headers
export CURLREVSHELL_RESPONSE_HEADERS_FILE=${TF}.json
WANT=$(<${TF}.want)
tap_subtest "Response headers set" subtest "$0" $LINENO

# vim: ft=sh
