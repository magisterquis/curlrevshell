#!/bin/ksh
#
# log.t
# Make sure logging works
# By J. Stuart McMurray
# Created 20251008
# Last Modified 20251212

. ./t/t.subr
. ./t/shmore.subr

tap_plan 1

GOT=$(echo -n | gorun -log /dev/stderr 2>&1 >/dev/null |
        jq -r 'select("Listener started" == .msg) | .fingerprint')
tap_like "$GOT" '^[0-9A-Za-z+/]{43}=$' "Fingerprint logged" "$0" $LINENO

# vim: ft=sh
