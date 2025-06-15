#!/bin/ksh
#
# basic_tests.t
# Make sure our code is up-to-date and doesn't have debug things.
# By J. Stuart McMurray
# Created 20250615
# Last Modified 20250615

set -uo pipefail

. t/shmore.subr

NTEST=6
tap_plan "$NTEST"

# Make sure we didn't leave any stray DEBUGs or TAP_TODOs lying about.
GOT=$(egrep -InR '(#|\*)[[:space:]]*()DEBUG' | sort -u)
tap_is "$GOT" "" "No files with DEBUG comments" "$0" $LINENO
GOT=$(egrep -In  'TAP_TODO[=]' t/*.t | sort -u)
tap_is "$GOT" "" "No TAP_TODO's" "$0" $LINENO

# These checks assume we're writing a Go program.
if [[ -f ./go.mod ]]; then
        # TMPD is where we'll put our temporary program
        TMPD=$(mktemp -td)
        trap 'rm -rf ${TMPD}; tap_done_testing' EXIT

        # Make sure we're not using MQD.
        GOT="$(go run . -h </dev/null 2>&1 |
                egrep 'MQD DEBUG PACKAGE LOADED$')"
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

        # Make sure we're using the latest Go as well.
        GOT="$(go list \
                -u \
                -f '{{if (and .Update .Update.Version) -}}
                        go {{.Version}} -> {{.Update.Version}}
                {{- end}}' \
                -m go)"
        tap_is "$GOT" "" "Latest Go version will be used" "$0" $LINENO
else
        tap_skip "Not a Go program" $((NTEST-2))
fi

# vim: ft=sh
