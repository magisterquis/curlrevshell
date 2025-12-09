#!/bin/ksh
#
# make.t
# Make sure our makefiles are all included.
# By J. Stuart McMurray
# Created 20241209
# Last Modified 20251209

set -euo pipefail

. t/shmore.subr

# Included makefiles we expect.
set -A FNS src/mk/*

# One test per included makefile
tap_plan "${#FNS[*]}"

# Included Makefiles
INCS=$(make -V MAKEFILE_LIST)

for FN in ${FNS[@]}; do
        # Make sure this makefile is imported.
        GOT=$(echo -E "$INCS" | fgrep -o "$FN" ||:)
        tap_is "$GOT" "$FN" "Submakefile $FN included" "$0" $LINENO
done

# vim: ft=sh
