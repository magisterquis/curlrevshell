#!/bin/ksh
#
# make.t
# Make sure our makefiles are all included.
# By J. Stuart McMurray
# Created 20241209
# Last Modified 20261017

set -euo pipefail

. t/shmore.subr

# Work out which make to use
MAKE=make
OS=$(uname -s)
if [[ "Darwin" == "$OS" || "Linux" == "$OS" ]]; then
        if type bmake >/dev/null 2>&1; then
                MAKE=bmake
        fi
fi

# Included makefiles we expect.
set -A FNS src/mk/*

# One test per included makefile
tap_plan "${#FNS[*]}"

# Included Makefiles, for OpenBSD make bmake, at least.
INCS=$($MAKE -V MAKEFILE_LIST -V .MAKE.MAKEFILES)
for FN in ${FNS[@]}; do
        # Make sure this makefile is imported.
        GOT=$(echo -E "$INCS" | grep -F -o "$FN" ||:)
        tap_is "$GOT" "$FN" "Submakefile $FN included" "$0" $LINENO
done

# vim: ft=sh
