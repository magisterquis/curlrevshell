#!/usr/bin/env perl
#
# hexevalify.pl
# Turn a perl script into hex in an environment variable, eval'd, in a function
# By J. Stuart McMurray
# Created 20240525
# Last Modified 20240602

use warnings;
use strict;
use File::Basename;

# envname is the name of the environment variable with the hexed script.
my $envname="HEXEVALIFY";

# Make sure we have a file to convert.
if (0 != $#ARGV) {
        print STDERR "Usage: $0 perlscript\n";
        exit 1;
}
open my $FH, "<$ARGV[0]" or die "open $ARGV[0]: $!";

# Figure out the function name.
my ($fn, undef, undef) = fileparse("$ARGV[0]", qr/\.[^.]*/) or
        die "Could not parse filename $ARGV[0]: $!";

# Slurp and hexify the script in question.
my $payload;
{
        local $/;
        defined($payload = <$FH>) or die "Read error: $!";
}
my $hex = unpack("H*", $payload);

# Emit a nice script in a f
print qq<function $fn { >;
print qq<$envname=$hex >;
print qq<perl -e 'eval(pack"H*",\$ENV{$envname});>;
print  q<die"Error: $@"if(""ne$@)' "$@"; }>;
print qq<\n>;
