#!/usr/bin/env perl
#
# changelog.t
# Make sure our changelog looks ok
# By J. Stuart McMurray
# Created 20261022
# Last Modified 20261022

use autodie;
use warnings;
use strict;

use feature 'signatures';

use Fcntl qw/O_RDONLY/;
use Test::More;
use Tie::File;

# Some entries are ok to be dupes.
my %ok_dupes = (
        "- Updated dependencies." => 1,
);

# Arrayify the changelog.
my $fn = "doc/changelog.md";
tie my @changelog, 'Tie::File', $fn, mode => O_RDONLY
        or die "Error opening $fn: $!";

# Collect entries.  Each entry starts with a hyphen and may be continued by
# lines starting with whitespace.
my @entries;
my $started = 0;
for (@changelog) {
        if (/^- /) { # New entry.
                push @entries, $_;
                $started = 1;
        } elsif ($started && /^  /) { # Continuation.
                $entries[$#entries] .= s/^  / /r;
        } else { # No longer in an entry.
                $started = 0;
        }
}

# Work out where sections start.
my @sstarts;
for my $i (0..$#changelog) {
        push @sstarts, $i-1 if $changelog[$i] =~ /^-+$/;
}

# We finally know how many tests we'll run.
plan tests => 3 + 1+@entries + 1+@sstarts;
isnt 0+@entries, 0, "Read changelog entries";
isnt 0+@sstarts, 0, "Found section headers";

# Check for dupes.
my %seen;
for my $i (0..$#entries) {
        my $n = $i + 1; # Human index
        my $entry = $entries[$i];
        ok ! exists $seen{$entry} || exists $ok_dupes{$entry},
                "Entry $n unique: $entry";
        $seen{$entry} = 1;
}

# Check for two spaces after each section.
sub two_blanks_before ($i) {
        return 2 <= $i && "" eq $changelog[$i-1] && "" eq $changelog[$i-2];
}
for my $sstart (@sstarts) {
        my $n = $sstart + 1; # Human index
        ok two_blanks_before($sstart),"Section has two leading spaces - ".
                "$changelog[$sstart] (line $n)";
}

# Make sure the first entry has a message about changes, etc.
my $m;
for my $i (0..$#changelog) {
        if ($changelog[$i] =~ /^-+$/) {
                $m = $i;
                last;
        }
}
my $snre = qr/^`([^`]+)`$/;
like $changelog[$m+1], qr/^The following/,      "Changes message exists";
like $changelog[$m-1], $snre,         "Fist section name found";
$changelog[$m-1] =~ $snre;
my $title = quotemeta($1 // "");
like $changelog[$m+4], qr/^go install.*$title$/, "Found go install line";

done_testing;
