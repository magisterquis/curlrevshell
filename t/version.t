#!/bin/sh
#
# version.t
# Make sure docs are consistent with this version of curlrevshell
# By J. Stuart McMurray
# Created 20241203
# Last Modified 20241226

set -e

. t/shmore.subr

tap_plan 6

# Tag we expect to use for installing
TAG="$(git branch --show-current)"
if [ "master" == "$WANT" ]; then
        # On the master branch, we should line up with the latest tag
        TAG="$(git describe --tags)"

fi
tap_isnt "$TAG" "" "Worked out our install tag" "$0" $LINENO

# Unfortunately, we'll just get (devel) here.  Better than nothing?
WANT="Welcome to curlrevshell version (devel)"
GOT="$(echo -n | go run . -no-timestamps 2>&1 | fgrep -o "$WANT")"
tap_is "$GOT" "$WANT" "Version looks ok" "$0" $LINENO

# Make sure that all of the go install URLs are for this branch.
subtest() {
        # Look for what looks like go install URLs
        MATCHES="$(find . -type f \
                \! -name '*.swp' \
                \! -path './t/*' \
                -exec egrep \
                        -no \
                        'github.com/magisterquis/curlrevshell@[^[:space:]]+' \
                        {} + |
                sort -u)"

        # Number of files is the number of subtest tests we'll run, plus a
        # check we got files at all
        tap_plan "$(( $(echo -n "$MATCHES" | egrep -v '^$' | wc -l) + 1 ))"

        # Make sure we got at least one file
        tap_isnt "$MATCHES" "" "Got files with the install path" "$0" $LINENO

        # Make sure each file is correct
        IFS='
'
        for MATCH in $MATCHES; do
                GOT="$(echo "$MATCH" | cut -f 2 -d @)"
                FILE="$(echo "$MATCH" | cut -f 1,2 -d :)"
                tap_is "$GOT" "$TAG" "$FILE is correct" "$0" $LINENO
        done
}
tap_subtest "Correct go install paths in docs" subtest "$0" $LINENO

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

# Make sure the top of the changelog has the right version
GOT="$(awk '5==NR' doc/changelog.md | cut -f 2 -d '`')"
tap_is "$GOT" "$TAG" "Changelog has correct tag" "$0" $LINENO

# vim: ft=sh
