#!/bin/ksh
#
# websocket.t
# Can we connect via a websocket?
# By J. Stuart McMurray
# Created 20260804
# Last Modified 20260807

set -euo pipefail

. t/shmore.subr
. t/t.subr

tap_plan 13

gorun |&
RET=$?
tap_is "$RET" "$?" "Go run started ok" "$0" $LINENO

# Hook up curlrevshell to FDs 4 (in) and 5 (out), as we'll need another
# oprocess.
exec 4>&p 5<&p

# Get connection lines.
TGAS=
while read -r -u5; do
        if [[ "$REPLY" == curl\ -sk* ]]; then
                TGAS=$REPLY
                break
        fi
done
if [[ -z "$TGAS" ]]; then
        tap_fail "Did not get a curl command" "$0 "$LINENO
        exit 1
fi
set -A TGASV $TGAS

# argv_like checks that argv[$1] regex-matches $2,
#
# Arguments:
# $1 - i in argv[i]
# $2 - regex
# $3 - What the element is, for test naming
# $4 - $LINENO
argv_is() {
        local _i=$1 _like=$2 _name=$3 _lineno=$4
        tap_like \
                "${TGASV[$_i]}" \
                "$_like" \
                "Default curl command $_name (argv[$_i]) looks ok" \
                "$0" "$_lineno"
}

# Make sure our curl command has the important bits.
argv_is 0 "^curl$"                       "name"         $LINENO
argv_is 1 "^-sk$"                        "short flags"  $LINENO
argv_is 2 "^--pinnedpubkey$"             "pubkey flag"  $LINENO
argv_is 3 "^sha256//[A-Za-z0-9+/]{43}=$" "pubkey hash"  $LINENO
argv_is 4 '^https://127\.0\.0\.1:\d+/c$' "HTTP address" $LINENO

# But make the URL a wss:// URL.
TGASV[4]=wss://${TGASV[4]#https://}

# Does our script have wss:// in it?
GOT=$(${TGASV[0]} ${TGASV[1]} ${TGASV[2]} ${TGASV[3]} ${TGASV[4]})
tap_like \
        "$GOT" \
        'wss:\/\/' \
        "Script requested with wss:// has wss://" \
        "$0" $LINENO

# Queue up a line for our websocket "shell" to get.
SHELL_INPUT="shell-input-$RANDOM"
print -r -u4 "$SHELL_INPUT"

# And make sure it's queued.
while read -r -u5; do
        if [[ "$REPLY" == "Buffering input until a shell connects..." ]]; then
                break
        fi
done

# Prep to connect via a websocket.
ADDR=${TGASV[4]%/c}           # Correct path

# Connect the shell via websockets, send "output" and get "input."
ID=id-$RANDOM
curl \
        --insecure \
        --no-buffer \
        --no-progress-meter \
        --pinnedpubkey "${TGASV[3]}" \
        --silent \
        --upload-file . \
        ${TGASV[4]%/c}/io/$ID \
        "$ADDR/$ID" |&
CPID=$!

# Wait for the shell to connect.
read -r -u5
tap_is \
        "$REPLY" \
        "[127.0.0.1] Connected: ID $ID" \
        "Shell connected" \
        "$0" $LINENO
read -r -u5
tap_is \
        "$REPLY" \
        "[127.0.0.1] Shell is ready to go!" \
        "Shell is ready to go!" \
        "$0" $LINENO

# Get our shell input, to which we'll reply with output.
read -pr
tap_is \
        "$REPLY" \
        "$SHELL_INPUT" \
        "Got shell input" \
        "$0" $LINENO

# Try to reply with output.
SHELL_OUTPUT="shell-output-$RANDOM"
print -pr "$SHELL_OUTPUT"
tap_is $? 0 "Sent shell output" "$0" $LINENO

# Did we get the shell output?
read -r -u5
tap_is \
        "$REPLY" \
        "$SHELL_OUTPUT" \
        "Got shell output" \
        "$0" $LINENO

# Close everything
exec 4>&-

# Everybody exit?
wait
tap_pass "All child processes exited"

# vim: ft=sh
