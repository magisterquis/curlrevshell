#!/bin/ksh
#
# compile_time_defaults.t
# Make sure compile-time defaults work
# By J. Stuart McMurray
# Created 202501010
# Last Modified 20251010

set -euo pipefail

. t/shmore.subr

tap_plan 4

# Work out some defaults.
TMPD=$(mktemp -d)
rmdir "$TMPD" # We're just using it for its name.
DEFCID=$TMPD/ctrl-i
DEFLOG=$TMPD/log.json
DEFSFF=$TMPD/files
DEFCBT=$TMPD/crs.tmpl
LDFLAGS="
        -X main.DefaultCtrlI=$DEFCID
        -X main.DefaultLog=$DEFLOG
        -X main.DefaultServeFilesFrom=$DEFSFF
        -X main.DefaultTemplate=$DEFCBT
"

# get_default reads the next line from the coprocess, which should be a flag's
# help text, and extracts the default value.
get_default() {
        local _got
        read -pr _got
        _got=${_got##*\(default \"}
        _got=${_got%\"\)}
        echo "$_got"
}

# Get the compile-time-set defaults.
GOTSFF=
GOTCBD=
GOTLOG=
GOTCID=
go run -ldflags "$LDFLAGS" . -h 2>&1 |&
while read -pr FLAG _; do
        case "$FLAG" in
                -ctrl-i)           GOTCID=$(get_default) ;;
                -log)              GOTLOG=$(get_default) ;;
                -serve-files-from) GOTSFF=$(get_default) ;;
                -template)         GOTCBT=$(get_default) ;;
        esac
done

# Did it work?
tap_is "$GOTCID" "$DEFCID" "-ctrl-i correct"           "$0" $LINENO
tap_is "$GOTLOG" "$DEFLOG" "-log correct"              "$0" $LINENO
tap_is "$GOTSFF" "$DEFSFF" "-serve-files-from correct" "$0" $LINENO
tap_is "$GOTCBT" "$DEFCBT" "-template correct"         "$0" $LINENO

# Make sure curlrevshell exits (or at least go run . does).
wait

# vim: ft=sh
