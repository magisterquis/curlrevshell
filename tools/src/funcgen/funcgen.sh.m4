#!/bin/sh
# funcgen.sh
# Shell functions generator
# By J. Stuart McMurray
# Created 20240527
# Last Modified 20240602

set -e

# Command-line parts passed to make
MAKECLEAN=
MAKEFLAGS="-f -"

# Variables to send to make
OUTFILE=
TEMPLATEFILE=

# Other random and sundry things.
PRINTOUT=                                   # Set to print the output file
TMPDIR="$(mktemp -d -t funcgen.XXXXXXXXXX)" # Where we stick temporary files
UBIT="@builtin"                             # Use built-in template

# Usage tells the user how to use, usefully, and exits.
function usage {
        cat <<_eof
Usage: $1 [-chxv] [-o output_file] [-t callback_template] funcdir [funcdir...]

Rolls a file suitable for use with -ctrl-i-file (by default) or
-callback-template (with -t) which loads shell functions from files in one or
more directories (funcdir) into the remote shell.

In each funcdir should be files with names ending in .sh containing shell
functions (i.e. function foo {...} or foo() {...}) or perl scripts with names
ending in .pl.  Files ending in .sh may also have code which executes when the
script is sent.  Files ending in .pl will be turned into shell functions with
the same name as the file, less the .pl.  Multiple files with the name name but
different extensions are a bad idea.

A template for use with -callback-template may be specified with -t.  This
will be passed through m4 -PEE, with the functions, escaped for inclusion in
single quotes, sent on stdin.  An example template may be printed with -x.  If
the special value $UBIT is passed via -t, a built-in template will be used.

Options:
  -c    Clean (remove) generated files from the funcdir(s)
  -h    Print this help
  -o filename
        Optional output filename, default is standard output
  -t callback_template
        Optional custom -callback-template template
  -v    Print commands as they're run
  -x    Print an example tempalte for use with -t
_eof
}

# atexit is run when the script terminates
function atexit {
        # If we're meant to write to stdout, do so
        if [ -n "$PRINTOUT" ]; then
                cat "$OUTFILE"
        fi
        # If we made a directory, delete it.
        if [ -d "$TMPDIR" ]; then
                rm -rf "$TMPDIR"
        fi
}

# print_example_template prints an example template for use with -t.
function print_example_template {
        cat <<'_eof'
m4_paste(callback.tmpl)m4_dnl
_eof
}

# Helper scripts, which we'll pass to the makefile in variables.
function print_hexevalify_pl {
        cat <<"_eof"
m4_paste(helpers/hexevalify.pl)m4_dnl
_eof
}

# Work out what we're building and how.
while getopts chio:t:vx name; do
        case "$name" in
                c) MAKECLEAN="clean"              ;;
                h) usage "$0"; exit 0             ;;
                o) OUTFILE="$OPTARG"              ;;
                t) TEMPLATEFILE="$OPTARG"         ;;
                v) MAKEFLAGS="$MAKEFLAGS -dl"     ;;
                x) print_example_template; exit 0 ;;
                ?) usage "$0"; exit 1             ;;
        esac
done
shift "$((OPTIND - 1))"

# Need at least one source directory
if [ -z "$*" ]; then
        echo "Need at least one functions files directory." >&2
        exit 4
fi

# Delete any files we've made on exit
trap "atexit" EXIT

# Work out which version of Make to use, and make sure we have it.
MAKE=make
OSNAME="$(uname -s)"
case "$OSNAME" in
        Linux|Darwin) MAKE=bmake                                       ;;
        OpenBSD)                                                       ;; # :)
        *) echo "Unexpected OS $OSNAME.  Hope we have BSD make..." >&2 ;;
esac
if ! type "$MAKE" >/dev/null; then
        echo "Make program $MAKE not found" >&2
        exit 3
fi

# If we're writing to stdout, we'll actually write to a temporary file and then
# print that to stdout.
if [ -z "$OUTFILE" ] && [ -z "$MAKECLEAN" ]; then
        PRINTOUT=yes
        OUTFILE="$TMPDIR/out"
fi

# If we're using a built-in template, make it actually a file.
if [ "$UBIT" = "$TEMPLATEFILE" ]; then
        TEMPLATEFILE="$TMPDIR/template"
        print_example_template > "$TEMPLATEFILE"
        touch -r "$0" "$TEMPLATEFILE"
fi

# All the real work is done by make.
HEXEVALIFY_PL="$(print_hexevalify_pl)" \
"$MAKE" \
        $MAKEFLAGS \
        SRCDIRS="$*" \
        OUTFILE="$OUTFILE" \
        TEMPLATEFILE="$TEMPLATEFILE" \
        $MAKECLEAN \
        <<"_eof_makefile"
m4_paste(funcgen.mk)m4_dnl
_eof_makefile

# vim: ft=sh
