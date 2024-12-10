#!/bin/sh
#
# version.t
# Make sure docs are consistent with this version of curlrevshell
# By J. Stuart McMurray
# Created 20241203
# Last Modified 20241210

set -e

. t/shmore.subr

tap_plan 5

# Unfortunately, we'll just get (devel) here.  Better than nothing?
WANT="Welcome to curlrevshell version (devel)"
GOT="$(echo -n | go run . -no-timestamps 2>&1 | fgrep -o "$WANT")"
tap_is "$GOT" "$WANT" "Version looks ok" "$0" $LINENO

# Make sure that all of the go install URLs are consistent.
GOT="$(find . -type f \
        \! -name '*.swp' \
        \! -path './t/*' \
        -exec egrep \
                -hor \
                'github.com/magisterquis/curlrevshell@[^[:space:]]+' {} + |
        sort -u)"
tap_unlike "$GOT" '\n.' "Consistent install path" "$0" $LINENO

# Make sure it's the current branch
GOT="$(echo "$GOT" | cut -f 2 -d @ | tail -n 1)"
WANT="$(git branch --show-current)"
if [ "master" == "$GOT" ]; then
        # On the master branch, we should line up with the latest tag
        GOT="$(git describe --tags)"

fi
tap_is "$GOT" "$WANT" "Correct install path" "$0" $LINENO

# Make sure the -h output in the README is correct
WANT="$(go run . -h 2>&1)"
GOT="$(awk '
/^Usage$/ { # For waiting until we have the usage section
        in_usage = 1
        next # No point in the rest
}
/^```$/ { # For noting code block boundaries
        # Only care once we get to the usage block
        if (! in_usage) {
                next
        }
        # Keep track of where we are in the code block
        nbt++
        # No need to print this line
        next
}
# Print anything in the Usage code block
1 == in_usage && 1 == nbt
' README.md)"
tap_is "$GOT" "$WANT" "Correct -h output in README" "$0" $LINENO

# Make sure the package versions in the readme are correct
GOT=$(grep 'go: downloading' README.md | cut -f 3- -d ' ' | sort -u)
WANT=$(cut -f 1-2 -d ' ' go.sum | grep -v go.mod  | sort -u)
tap_is "$GOT" "$WANT" "Correct downloaded modules in README" "$0" $LINENO

# vim: ft=sh
