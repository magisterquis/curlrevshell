#!/bin/ksh
#
# log.t
# Make sure logging works
# By J. Stuart McMurray
# Created 20251008
# Last Modified 20261017

. ./t/t.subr

tap_plan 2

# Extract the fingerprint from the logs, piped to perl.
try() {
        gorun -log /dev/stderr 2>&1 >/dev/null |
        perl -MJSON::PP -ne '
                my $j = decode_json $_ or die "decode_json: $!";
                if ("Listener started" eq $j->{msg}) {
                        print "$j->{fingerprint}\n";
                        exit 0;
                }'
}
try |&

# Did it work?
read -pr GOT
tap_like "$GOT" '^[0-9A-Za-z+/]{43}=$' "Fingerprint logged" "$0" $LINENO

# Everybody finished?
exec 3>&p; exec 3>&-
wait
tap_pass "All child processes exited"

# vim: ft=sh
