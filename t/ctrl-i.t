#!/bin/sh
#
# ctrl-i.t
# Tests for inserting things
# By J. Stuart McMurray
# Created 20241204
# Last Modified 20241226

set -e

. t/shmore.subr

tap_plan 2

# find_tab_list grabs lines from Ctrl+Jing the -ctrl-i file $1
find_tab_list() {
        while printf '\012'; do sleep .1; done |
        go run . -ctrl-i "$1" 2>/dev/null | 
        perl -e '
                while (<>) {last if /Would have sent the following \d+ bytes/}
                while (<>) {
                        last if /Would have sent the following \d+ bytes/;
                        if (/tab_list/) { s/[\r\n]+//g; print; last; }
                }
        '
}

# Make sure a ctrl-i file which shouldn't print tab_list doesnt.
tap_isnt \
        "$(find_tab_list t/testdata/empty.subr)" \
        "" \
        "Generated tab_list by default" \
        "$0" $LINENO

tap_is \
        "$(find_tab_list t/testdata/notablist.subr)" \
        "" \
        "No tab_list with NOTABLIST" \
        "$0" $LINENO

# vim: ft=sh
