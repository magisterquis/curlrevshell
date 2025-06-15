#!/bin/ksh
#
# branch_name.t
# Make sure we're using the right branch name
# By J. Stuart McMurray
# Created 20250615
# Last Modified 20250615

set -euo pipefail

. t/shmore.subr
. t/t.subr

tap_plan 1

MODULE=./internal/currentversion

GOT=$(<$MODULE/current_branch)
WANT=$(current_git_branch)
tap_is "$GOT" "$WANT" "Branch name up-to-date" "$0" $LINENO
if [[ "$GOT" != "$WANT" ]]; then
        tap_diag "Probably need to re-run"
        tap_diag "        go generate -x $MODULE"
fi

# vim: ft=sh

