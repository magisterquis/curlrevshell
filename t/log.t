#!/bin/ksh
#
# log.t
# Make sure logging works
# By J. Stuart McMurray
# Created 20251008
# Last Modified 20260304

. ./t/t.subr
. ./t/shmore.subr

tap_plan 1

GOT=$(echo -n | gorun -log /dev/stderr 2>&1 >/dev/null |
        perl -MJSON::PP -ne '
                my $j = decode_json $_ or die "decode_json: $!";
                if ("Listener started" eq $j->{msg}) {
                        print $j->{fingerprint};
                }
        ')
tap_like "$GOT" '^[0-9A-Za-z+/]{43}=$' "Fingerprint logged" "$0" $LINENO

# vim: ft=sh
