#!/bin/ksh
#
# basic_tests.t
# Make sure our code is up-to-date and doesn't have debug things.
# By J. Stuart McMurray
# Created 20250818
# Last Modified 20250818

set -euo pipefail

. t/shmore.subr

NTEST=8
tap_plan "$NTEST"

# OK_DEBUG and OK_TODO, if exant, contain grep output lines to be ignored
# in the first two tests.
OK_DEBUG=t/testdata/basic_tests/debug_ok
OK_TODO=t/testdata/basic_tests/todo_ok

# Make sure we didn't leave any stray DEBUGs or TAP_TODOs lying about.
GOT=$(grep -EInR '(#|\*|^)[[:space:]]*()DEBUG' | sort -u |
        grep -Ev '^t/shmore.subr:[[:digit:]]+:' |
        grep -Ev "^$OK_DEBUG:[[:digit:]]+" ||:)
if [[ -f "$OK_DEBUG" ]]; then
        GOT=$(print -r "$GOT" | grep -Fvf "$OK_DEBUG" ||:);
fi
tap_is "$GOT" "" "No files with unexpected DEBUG comments" "$0" $LINENO
GOT=$(grep -EInR '(#|\*|^)[[:space:]]*()TODO' | sort -u |
        grep -Ev '^(\.git/hooks/[^:]+\.sample|t/shmore.subr):[[:digit:]]+:' |
        grep -Ev "^$OK_DEBUG:[[:digit:]]+" ||:)
if [[ -f "$OK_TODO" ]]; then
        GOT=$(print -r "$GOT" | grep -Fvf "$OK_TODO" ||:);
fi
tap_is "$GOT" "" "No files with unexpected TODO comments" "$0" $LINENO
GOT=$(grep -EIn  'TAP_TODO[=]' t/*.t | sort -u ||:)
tap_is "$GOT" "" "No TAP_TODO's" "$0" $LINENO

# These checks assume we're writing a Go program.
if [[ -f ./go.mod ]]; then
        # TMPD is where we'll put our temporary program
        TMPD=$(mktemp -d)
        trap 'rm -rf ${TMPD}; tap_done_testing' EXIT

        # Make sure we're not using MQD.
        GOT="$(go run . -h </dev/null 2>&1 |
                grep -E 'MQD DEBUG PACKAGE LOADED$' ||:)"
        tap_is "$GOT" "" "Not using github.com/magisterquis/mqd" "$0" $LINENO

        # Should get happy help output.  We can't use go run here because it
        # doesn't properly propagate the exit status.
        go build -o "$TMPD/tb"
        "$TMPD/tb" -h 2>/dev/null
        tap_is $? 0 "Running with -h exits happily" "$0" $LINENO

        # Make sure we don't need to update anything.
        GOT="$(go list \
                -u \
                -f '{{if (and (not (or .Main .Indirect)) .Update)}}
                        {{- .Path}}: {{.Version}} -> {{.Update.Version -}}
                {{end}}' \
                -m all)"
        tap_is "$GOT" "" "Packages up-to-date" "$0" $LINENO
        # Idea stolen from https://github.com/fogfish/go-check-updates

        # Check for newer versions of tools
        GOT="$(go list \
                -u \
                -f '{{if .Update}}
                        {{- .Path}}: {{.Version}} -> {{.Update.Version -}}
                {{end}}' \
                -m $(go list -f '{{.Module.Path}}' tool))"
        tap_is "$GOT" "" "Tools up-to-date" "$0" $LINENO

        # Make sure we're using the latest Go as well.
        GOT="$(go list \
                -u \
                -f '{{if (and .Update .Update.Version) -}}
                        go {{.Version}} -> {{.Update.Version}}
                {{- end}}' \
                -m go)"
        tap_is "$GOT" "" "Latest Go version will be used" "$0" $LINENO
else
        tap_skip "Not a Go program" $((NTEST-3))
fi

# vim: ft=sh
