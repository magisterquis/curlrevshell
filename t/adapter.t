#!/usr/bin/env perl
#
# adapter.t
# Can we connect via an adapter?
# By J. Stuart McMurray
# Created 20260809
# Last Modified 20260821

use autodie;
use strict;
use warnings;

use feature 'signatures';

use File::Temp;
use IO::Socket::UNIX;
use IPC::Open2;
use JSON::PP;
use Test::More tests => 10;

# Directory for our socket.
my $id = "id-" . int rand 1000000;
my $tdir  = File::Temp->newdir();# or die "newdir: $!";
my $spath = "$tdir/s";

# Spawn curlrevshell.
my $pid = open2 my $crs_out, my $crs_in,
        "go", "run", ".",
                "-adapter-socket", $spath,
                "-listen-address", "127.0.0.1:0",
                "-no-timestamps",
                "-tls-certificate-cache", ""
        or die "starting curlrevshell: $!";
select((select($crs_in),  $|=1)[0]);
select((select($crs_out), $|=1)[0]);

# Wait for adapter path.
my $got_path;
while (<$crs_out>) {
        next unless /^Listening for adapters on (.*)/;
        $got_path = $1;
        last;
}
is $got_path, $spath, "Adapter path correct";

# Connect as an adapter.
my $s = IO::Socket::UNIX->new($spath) or die "connecting adapter: $!";
is $s->peerpath, $spath, "Connected as adapter";

# Request a bidirectional shell.
my $req = encode_json {
        ConnType => "shell-stream",
        Args     => {
                Direction => "in/out",
                ID        => $id,
                Tag       => $0,
        },
};
print $s $req or die "sending shell request: $!";

# Get a response.
defined(my $res = <$s>) or die "reading response: $!";
is $res, "{}\n", "Got happy response to shell request";

# read_line reads the next line from the given handle, chomped.
sub read_line ($h) {
        defined(my $l = <$h>) or die "reading from $h: $!";
        chomp $l;
        return $l;
}



# Should get a connected and a shell ready to go.
my $line = "";
$line = read_line($crs_out) while $line !~ /Connected/;
is $line,               "[$0] Connected: ID $id",     "Connected line correct";
is read_line($crs_out), "[$0] Shell is ready to go!", "Shell is ready to go!";

# Send a line to the shell.
my $sil = "Shell input line";
print $crs_in "$sil\n" or die "sending shell input: $!";
is read_line($s), $sil, "Read line sent to the shell";

# Send a line from the shell.
my $sol = "Shell output line";
print $s "$sol\n" or die "sending shell output: $!";
is read_line($crs_out), $sol, "Read line sent from the shell";

# Disconnect the adapter.
close $s;
is read_line($crs_out), "[$0] Connection closed", "Connection closed";
is read_line($crs_out), "[$0] Shell is gone :(",  "Shell is gone :(";

# Stop curlrevshell.
close $crs_in or die "closing curlrevshell's input: $!";
waitpid $pid, 0;
is $?, 0, "Curlrevshell exited happily";

done_testing;
