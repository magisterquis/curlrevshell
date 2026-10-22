#!/bin/ksh
#
# welcome_template.t
# Can we make a custom welcome message?
# By J. Stuart McMurray
# Created 20261021
# Last Modified 20261021

. t/t.subr

tap_plan 5

gorun -template t/testdata/welcome_template/welcome.tmpl |&

# line_like reads a line from the coroutine and checks that it matches the
# regex in $1
#
# Arguments:
# $1 - Regex line must match
# $2 - Test name
# $2 - $LINENO
line_like() {
        typeset _regex=$1 _name=$2 _lineno=$3 _line
        read -pr _line
        tap_like "$_line" "$_regex" "$_name" "$0" "$_lineno"
}

line_like '^Welcome message start$'        "Got message start"  $LINENO
line_like '^Version: .+'                   "Got version"        $LINENO
line_like '^ListenAddress: 127.0.0.1:\d+$' "Got listen address" $LINENO
line_like '^Welcome message end$'          "Got message end"    $LINENO

# All done
exec 3>&p; exec 3>&-
wait
tap_pass "All child processes exited"

tap_done_testing

# vim: ft=sh
