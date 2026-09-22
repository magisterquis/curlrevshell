#!/usr/bin/env perl
#
# godoc.t
# Make sure offline-friendly Go docs are up-to-date
# By J. Stuart McMurray
# Created 20260922
# Last Modified 20260922

use autodie;
use strict;
use warnings;

use File::Find;
use Test::More;
use Tie::File;

# Work out which directories have non-test Go sources.
my %dirs;
sub check {
        my $dir = $File::Find::dir;
        return if "." eq $dir;
        return unless /(?<!_test)\.go$/;
        $dirs{$dir} = 1;
}
find \&check, ".";

# Should have gotten quite a few.
my $nd = keys %dirs;
plan tests => 1 + (2*$nd);
cmp_ok $nd, '>', 0, "Found Go source directores";

# All of the godoc files correct?
for my $dir (sort keys %dirs) {
        SKIP: {
                my $fn = "$dir/godoc";
                # Do we have the file?
                unless (ok -f $fn, "$fn exists") {
                        skip "$fn missing", 1;
                }
                # Make sure the file matches the generated docs;
                tie my @got, "Tie::File", $fn, autochomp => 0  or die "open $fn: $!";
                my @want = `go doc -all $dir`;
                is_deeply \@got, \@want, "$fn correct";
        };
}

done_testing;
