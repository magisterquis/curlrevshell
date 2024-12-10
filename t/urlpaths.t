#!/usr/bin/env perl
#
# urlpaths.t
# Make sure we can set URL paths
# By J. Stuart McMurray
# Created 20241205
# Last Modified 20241205

use warnings;
use strict;

use Test::More tests => 6;

my $want_in     = "testitest";
my $want_out    = "testotest";
my $want_script = "testctest";

# Don't wait for curlrevshell to die.
$SIG{CHLD} = 'IGNORE';

open my $H, "-|", <<_eof or die "Starting curlrevshell: $!";
while printf '\\015'; do sleep .1; done |
go run -ldflags '-X main.URLPathIn=$want_in
        -X main.URLPathOut=$want_out
        -X main.URLPathScript=$want_script' \\
. -listen-address 127.0.0.1:0 2>&1
_eof

$|=1;

# Make sure we get a good listen address.
my $curl_cmd;
my $got_script;
my $ipaddr;
while (<$H>) {
        next unless m,curl -sk.*(https://127.0.0.1:\d+)/(\S+),;
        $curl_cmd   = $&;
        $got_script = $2;
        $ipaddr     = $1;
        last;
}
isnt $curl_cmd,   "",           "Got curl command";
isnt $ipaddr,     "",           "Got IP address"; 
is   $got_script, $want_script, "Script path";

# Grab the script from the address
my $curl_out = `$curl_cmd`;
isnt $curl_out, "", "Got callback script";

# Make sure we get input and output.
my @ms = $curl_out=~ m,$ipaddr/([^/]+)/,g;
is $ms[0], $want_in,  "Input path";
is $ms[1], $want_out, "Output path";

done_testing;
