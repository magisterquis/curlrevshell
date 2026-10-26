#!/bin/ksh
#
# tmplfuncs.t
# Test template functions themselves
# By J. Stuart McMurray
# Created 20250830
# Last Modified 20261010

set -euo pipefail

. t/t.subr

tap_plan 2

TMPD=$(mktemp -d)
TMPL="$TMPD/crs.tmpl"
trap 'rm -rf "$TMPD"; tap_done_testing' EXIT

# Template with MatchRE and Env.
export CRS_ENV_KEY="CRS_ENV_VAL-$RANDOM$RANDOM"
cat >"$TMPL" <<'_eof'
{{- define "callback" -}}
TEST: {{ if matchre `\d+` `abc1def` }}MatchRE Worked{{ end -}}
{{- if matchre `\d+` `abcdef` }}MatchRE Failed{{ end }}
{{ env "CRS_ENV_KEY" }}
{{- end -}}
_eof
gorun \
        -no-timestamps \
        -template "$TMPL" |&
# Does MatchRE work?
while read -pr; do
        if [[ "$REPLY" == TEST:* ]]; then
                break
        fi
done
WANT="TEST: MatchRE Worked"
tap_is "$REPLY" "$WANT" "MatchRE worked" "$0" $LINENO
# Does Env work?
read -pr
tap_is "$REPLY" "$CRS_ENV_KEY" "Env worked" "$0" $LINENO

# Don't need curlrevshell anymore.
exec 3>&p; exec 3>&-
wait

# vim: ft=sh
