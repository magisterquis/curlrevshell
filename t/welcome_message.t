#!/bin/ksh
#
# welcome_message.t
# Make sure the welcome message is correct
# By J. Stuart McMurray
# Created 20250615
# Last Modified 20250615

set -euo pipefail

. ./t/shmore.subr
. ./t/t.subr

FN=$0       # This file, for plike
GOTLN=1     # Read line number, for plike

# Current branch name
BRANCH=$(git rev-parse --abbrev-ref HEAD)

tap_plan 11 # One for each welcome message line

# plike reads a line from the coprocess and checks it against $1 with tap_like.
#
# Arguments:
# $1 - A regex against which to match the read line
# $2 - The line number from which plike was called
function plike {
        read -p
        tap_like "$REPLY" "$1" "Line $((GOTLN++)) correct" "$FN" "$2"
}

# Start the server serving test files
gorun -serve-files-from ./t -no-timestamps |&

CURLRESTART='^curl -sk '\
'--pinnedpubkey sha256//[a-z0-9A-Z+/]{43}= '\
'https://127.0.0.1:\d+$' 
plike "^Welcome to curlrevshell version \(devel\) \($BRANCH branch\)$" $LINENO
plike '^Listening on 127.0.0.1:\d+$'                                 $LINENO
plike '^To get files from ./t:$'                                     $LINENO
plike '^$'                                                           $LINENO
plike "$CURLRESTART\$"                                               $LINENO
plike '^$'                                                           $LINENO
plike '^To get a shell:$'                                            $LINENO
plike '^$'                                                           $LINENO
plike "$CURLRESTART/c | /bin/sh\$"                                   $LINENO
plike '^$'                                                           $LINENO

# Wait for curlrevshell to finish
exec 9>&p; exec 9>&-; wait
tap_is "$?" 0 "Curlrevshell exited happily" "$0" $LINENO

# vim: ft=sh
