#!/usr/bin/env perl
#
# tmplfuncs_docs.t
# Make sure tmplfuncs' docs are correct
# By J. Stuart McMurray
# Created 20250613
# Last Modified 20261008

use autodie;
use strict;
use warnings;

use Fcntl qw/O_RDONLY/;
use List::Util qw/first/;
use Test::More tests => 11;
use Tie::File;

use feature 'signatures';

my $libdir = "./lib/crstemplate/tmplfuncs";

# idx returns the index of what in where, or undef if not found.
sub idx ($what, @where) { first { $what eq $where[$_] } 0..$#where; }

# Get latest godoc
chomp(my @current_godoc = `go doc -all $libdir`);
is   $?,               0, "go doc ran happily";
isnt @current_godoc+0, 0, "Got docs from go doc";

# Is godoc.txt current?
my $fn = "$libdir/godoc.txt";
tie my @godoc_txt, 'Tie::File', $fn, mode => O_RDONLY or die "opening $fn: $!";
is_deeply \@godoc_txt, \@current_godoc, "Docs in godoc.txt current";

# But we don't want a trailing newline in the README.
pop @current_godoc while ("" eq $current_godoc[-1]);

# Extract the godoc from the README.
$fn = "$libdir/README.md";
tie my @readme, 'Tie::File', $fn, mode => O_RDONLY or die "opening $fn: $!";
my $start = idx '```text', @readme;
my $end   = idx '```',     @readme;
isnt $start, undef, "Found start of docs in README.md";
isnt $end,   undef, "Found end of docs in README.md";

# Is the godoc in the README correct?
my @readme_docs = @readme[$start+1..$end-1];
is_deeply \@readme_docs, \@current_godoc, "Docs in README.md correct";

# Does the godoc in the README end in not a newline.
isnt $readme_docs[-1], "\n", "Docs in README.md don't end in newline";

# Find the table of functions in template.md.
$fn="doc/template.md";
tie my @tmpl_md, 'Tie::File', $fn, mode => O_RDONLY or die "opening $fn: $!";
$start = idx "Functions", @tmpl_md;
isnt $start, undef, "Found functions section in template.md";
$start += 5;
like $tmpl_md[$start], qr/^\S+\s+\|\s+/,
        "Found start of functions table in tmplate.md";
$end = idx '', @tmpl_md[$start..$#tmpl_md];
isnt $end, undef, "Fond end of functions table in template.md";
$end += $start - 1;

# Get the list of functions in the docs and from go doc.
my @got  = map {s/\s.*//r} @tmpl_md[$start..$end];
my @want = map {/^func ([^(]+)/ ? lc $1 : undef } grep /^func /, @current_godoc;
is_deeply \@got, \@want, "All template functions listed in template.md";

done_testing;
