#!/bin/sh
#
# version.t
# Make sure we print a version number
# By J. Stuart McMurray
# Created 20241203
# Last Modified 20250112

set -e

. t/shmore.subr

tap_plan 2

# Unfortunately, we'll just get (devel) here.  Better than nothing?
WANT="Welcome to curlrevshell version (devel)"
GOT="$(echo -n | go run . -no-timestamps 2>&1 | fgrep -o "$WANT")"
tap_is "$GOT" "$WANT" "Version looks ok" "$0" $LINENO

# Make sure we don't need to update anything.
tap_is \
        "$(go list -u \
                -f '{{if (and (not (or .Main .Indirect)) .Update)}}
                        {{- .Path}}: {{.Version}} -> {{.Update.Version -}}
                {{end}}' \
                -m all)" \
        "" \
        "Packages up-to-date" \
        "$0" $LINENO
# Idea stolen from https://github.com/fogfish/go-check-updates

# vim: ft=sh
