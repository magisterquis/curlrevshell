#!/usr/bin/env perl
# pant.pl
# Knockoff of netstat -ant
# By The Author
# Created 20240602
# Last Modified 20240602

use strict;

sub pa {
        return join(".", reverse(map {unpack "C"}
                        split(//, pack("H*", shift)))) .
        ":" .
        unpack("S>", pack("H*", shift));
}

open my $H, "</proc/net/tcp" or die "Open: $!";

while (<$H>) {
        next unless /
                ^\s+\d+:\s
                ([0-9A-F]{8}):([0-9A-F]{4})\s
                ([0-9A-F]{8}):([0-9A-F]{4})\s
                ([0-9A-F]{2})\s 
                /x;
        printf "%2s %21s %21s %s\n", $5, pa($1, $2), pa($3, $4);
}

