#!/bin/ksh
#
# version.t
# Make sure docs are consistent with this version of curlrevshell
# By J. Stuart McMurray
# Created 20241203
# Last Modified 20260103

set -euo pipefail

. t/shmore.subr
. t/t.subr

tap_plan 8

# Tag we expect to use for installing
TAG="$(git branch --show-current)"
if [ "master" == "$TAG" ]; then
        # On the master branch, we should line up with the latest tag
        TAG="$(git describe --tags)"

fi
tap_isnt "$TAG" "" "Worked out our install tag" "$0" $LINENO

# Unfortunately, we'll just get (devel) plus maybe a branch here.  Better than
# nothing?
WANT="Welcome to curlrevshell version (devel)"
if [[ "master" != $(current_git_branch) ]]; then
        WANT="$WANT ($(current_git_branch) branch)"
fi
GOT=$(gorun -no-timestamps </dev/null 2>&1 |
        egrep '^Welcome to curlrevshell version' ||:)
tap_is "$GOT" "$WANT" "Version looks ok" "$0" $LINENO

# Make sure that all of the go install URLs are for this branch.
subtest() {
        # These should all be using "latest" if we're on the master branch.
        local _want=$TAG
        if [[ "master" = $(current_git_branch) ]]; then
                _want=latest
        fi

        PATTERN='github.com/magisterquis/curlrevshell@[^[:space:]]+'
        # Look for what looks like go install URLs
        LINES="$(find . -type f \
                \! -name '*.m4' \
                \! -name '*.swp' \
                \! -path './t/*' \
                -exec egrep \
                        -n \
                        "$PATTERN" \
                        {} + |
                sort -u)"

        # Number of files is the number of subtest tests we'll run, plus a
        # check we got files at all
        tap_plan "$(( $(echo -n "$LINES" | egrep -v '^$' | wc -l) + 1 ))"

        # Make sure we got at least one file
        tap_isnt "$LINES" "" "Got files with the install path" "$0" $LINENO

        # Make sure each file is correct
        IFS='
'
        set +o braceexpand
        for LINE in $LINES; do
                FILE="$(print -r "$LINE" | cut -f 1,2 -d :)"
                GOT="$( print -r "$LINE" | egrep -o "$PATTERN" |
                        cut -f 2 -d @)"
                # The changelog is special.  On the master branch, the go get
                # should actually be dev.
                local _linewant=$_want
                if [[ "$FILE" = ./doc/changelog.md:* &&
                        "master" == $(current_git_branch) ]]; then
                        _linewant=dev
                fi
                tap_is "$GOT" "$_linewant" "$FILE is correct" "$0" $LINENO
        done
        set +o braceexpand
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
# Credit to https://stackoverflow.com/questions/64371466/with-go-list-how-to-list-only-go-modules-used-in-the-binary#64390523
GOT=$(grep 'go: downloading' README.md | cut -f 3-4 | sort -u)
WANT=$(go list -deps -f '
        {{- define "M"}}go: downloading {{.Path}} {{.Version}}{{end -}}
        {{- with .Module -}}
                {{- if not .Main -}}
                        {{- if .Replace -}}
                                {{template "M" .Replace}}
                        {{- else -}}
                                {{template "M" .}}
                        {{- end -}}
                {{- end -}}
        {{- end -}}
' | sort -u)
tap_is "$GOT" "$WANT" "Correct downloaded modules in README" "$0" $LINENO

# Make sure the top of the changelog has the right version
GOT="$(egrep -B1 '^=+$' doc/changelog.md |
        egrep '^`' |
        cut -f 2 -d '`' |
        head -n 1)"
tap_is "$GOT" "$TAG" "Changelog has correct tag" "$0" $LINENO

# Make sure the welome message in the README at least has the right branch for
# branches, or right tag for master.
WELCOME=$(grep 'Welcome to curlrevshell version' README.md)
BRANCH=$(current_git_branch)
if [[ "$BRANCH" = master ]]; then
        GOT=$(print -r "$WELCOME" | cut -f 6 -d ' ')
        WANT="$TAG"
        tap_is \
                "$GOT" "$WANT" \
                "Version in welcome message in README correct" \
                "$0" $LINENO
else
        GOT=$(print -r "$WELCOME" | cut -f 7- -d ' ')
        WANT="($BRANCH branch)"
        tap_is \
                "$GOT" "$WANT" \
                "Branch name in welcome message in README correct" \
                "$0" $LINENO
fi

# Make sure there's no replace directives in go.mod.
GOT=$(egrep ^replace go.mod ||:)
tap_is "$GOT" "" "No replace directives in go.mod" "$0" $LINENO

# vim: ft=sh
