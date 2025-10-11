#!/bin/ksh
#
# tmplfuncs.t
# Test template functions themselves
# By J. Stuart McMurray
# Created 20250830
# Last Modified 20251011

set -euo pipefail

. t/shmore.subr
. t/t.subr

tap_plan 1

TMPD=$(mktemp -d)
TMPL="$TMPD/crs.tmpl"
trap 'rm -rf "$TMPD"; tap_done_testing' EXIT

# Does MatchRE work?
cat >"$TMPL" <<'_eof'
{{- define "callback" -}}
TEST: {{ if matchre `\d+` `abc1def` }}MatchRE Worked{{ end -}}
{{- if matchre `\d+` `abcdef` }}MatchRE Failed{{ end }}
{{- end -}}
_eof
gorun \
        -no-timestamps \
        -template "$TMPL" |&
while read -pr; do
        if [[ "$REPLY" == TEST:* ]]; then
                break
        fi
done
WANT="TEST: MatchRE Worked"
tap_is "$REPLY" "$WANT" "MatchRE worked" "$0" $LINENO

# Don't need curlrevshell anymore.
exec 3>&p; exec 3>&-
wait

# vim: ft=sh
