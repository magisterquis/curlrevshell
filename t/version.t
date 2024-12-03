#!/bin/sh
#
# version.t
# Make sure we print a version number
# By J. Stuart McMurray
# Created 20241203
# Last Modified 20241203

set -e

. t/shmore.subr

tap_plan 2

# Make sure we can print a version without dying funny.  We'd like to test
# the version, but we'll just get (devel).
go run . -version >/dev/null
tap_ok $? "-version ran ok" "$0" $LINENO

# Unfortunately, we'll just get (devel) here.  Better than nothing?
GOT="$(go run . -version 2>&1)"
tap_is "$GOT" "Version: (devel)" "Version looks ok" "$0" $LINENO

# vim: ft=sh
