#!/usr/bin/env perl
#
# tmplfuncs_templatefuncs.t
# Make sure we got all of the template functions in the functions map.
# By J. Stuart McMurray
# Created 20251011
# Last Modified 20260819

use autodie;
use strict;
use warnings;

use Test::More;

# Package with the template functions.
my $pkg = "./lib/crstemplate/tmplfuncs";

# Get the list of defined functions.
my @def_funcs = map {/^func ([A-Z][^\[\(]+).*/ ? $1 : ()} `go doc -short $pkg`;

# Four tests, plus one for each function.
plan tests => 4+@def_funcs;

# Though, this does require we actually have functions.
isnt 0+@def_funcs, 0, "Got list of exported functions";

# Work out what's in the map.
chomp(my @fmap = `go doc $pkg.TemplateFuncs`);

# First line is package name, second is blank.
@fmap = @fmap[2..$#fmap];

# Third line should be the declaration.
is shift(@fmap), "var TemplateFuncs = template.FuncMap{",
        "Got map declaration";

# Next several should be the contents.
my @map_funcs;
my $line;
for (@fmap) {
        $line = $_;
        # Give up when we hit the closing brace.
        last if '}' eq $line;
        unless (/^\t"([^"]+)":\s+([^,]+),$/) {
                diag "Invalid line in map: $line";
                last;
        }
        # Note what we've got.
        push @map_funcs, $2;
        # Names should be predictable.
        is $1, lc($2), "Exposed name correct: $2 -> $1";
}
is $line, "}", "Got to end of map";

# Should have the same names in the same order.
is @map_funcs, @def_funcs, "Correct functions in the map";

done_testing;
