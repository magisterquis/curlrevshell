#!/usr/bin/env perl
#
# urlpaths.t
# Make sure we can set URL paths
# By J. Stuart McMurray
# Created 20241205
# Last Modified 20261010

use autodie;
use strict;
use warnings;

use IPC::Cmd qw/can_run/;
use IPC::Open2;
use Test::More tests => 7;

$|=1;

my $want_in     = "testitest";
my $want_out    = "testotest";
my $want_script = "testctest";

# Can't do anything without curl.
my $curl = can_run("curl") // "";
isnt $curl, "", "Curl found";
exit 2 if "" eq $curl;

# Start curlrevshell
my $pid = open2 my $chld_out, my $chld_in, <<_eof
go run -ldflags '
        -X main.URLPathIn=$want_in
        -X main.URLPathOut=$want_out
        -X main.URLPathScript=$want_script
' . -listen-address 127.0.0.1:0 -tls-certificate-cache ''
_eof
        or die "starting curlrevshell: $!";

# Make sure we get a good listen address.
my $curl_cmd;
my $got_script;
my $ipaddr;
while (<$chld_out>) {
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

# Kill curlrevshell
close $chld_in or die "Closing curlrevshell's stdin $!";
1 while -1 != waitpid(-1, 0);

done_testing;
