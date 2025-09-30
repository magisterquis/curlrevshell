#!/bin/ksh
#
# template.t
# Make sure template things work as expected
# By J. Stuart McMurray
# Created 20241211
# Last Modified 20250615

set -e

. t/shmore.subr
. t/t.subr

tap_plan 13

# We'll need a temporary template and a directory for the templates from
# doc/template.md
TMPLF=$(mktemp -t  test.tmpl.XXXXXXXXXX)
TMPLD=$(mktemp -td test.tmpl.d.XXXXXXXXXX)
trap 'rm -r "$TMPLF" "$TMPLD"; tap_done_testing' EXIT

# Make sure we're not using html/template anywhere.
GOT="$(find . -type f \
        \! -name '*.swp' \
        \! -path './t/*' \
        -exec grep html/template {} + |
        egrep -v '^Binary file \./[^[:space:]]+ matches$' ||:
)"

tap_is "$GOT" "" "Not using html/template" "$0" $LINENO

# Make sure we get a warning if we use -callback-template.
GOT="$(gorun -callback-template ./doesnotexist </dev/null | cat)"
tap_like \
        "$GOT" \
        '(?s:-callback-template is.*'\
'going away eventually.*'\
'Use -template instead)' \
        "Deprecation warning for -callback-template" \
        "$0" $LINENO

# Make sure we get a warning if our template doesn't exist.
gorun -template ./doesnotexist |&
GOT=$(awk '3 == NR {print $0; exit}' <&p | cut -f 2- -d ' ')
exec 9>&p; exec 9>&-
tap_like \
        "$GOT" \
        'Error generating callback one-liners: executing callback subtemplate for 127.0.0.1:\d+: adding custom templates: reading template: open ./doesnotexist: no such file or directory' \
        "Warning for missing template file" \
        "$0" $LINENO

# Make sure we get a warning if we don't have subtemplates at all.
cat >$TMPLF <<_eof
No subtemplates at all :(
_eof
gorun -template "$TMPLF" |&
for i in `jot 3`; do read -p; done
tap_like \
        "$REPLY" \
        '^[0-9:.]+ Error generating callback one-liners: executing callback subtemplate for 127.0.0.1:\d+: adding custom templates: no subtemplates' \
        "Warning for old-style template file" \
        "$0" $LINENO
read -p
tap_like \
        "$REPLY" \
        '^[0-9:.]+ \tYou probably need {{define "callback"}} \.\.\. {{end}} around your template\.' \
        "Helpful message after old-style template file" \
        "$0" $LINENO
exec 9>&p; exec 9>&-; wait

# Make sure we get a warning if we're using a non-subtemplate template.
cat >$TMPLF <<_eof
{{define "script"}}dummy{{end}}
Extra data
_eof
gorun -template "$TMPLF" |&
for i in `jot 3`; do read -p; done
tap_like \
        "$REPLY" \
        '^[0-9:.]+ Error generating callback one-liners: executing callback subtemplate for 127.0.0.1:\d+: non-subtemplate template data found' \
        "Warning for template with outside-subtemplate data" \
        "$0" $LINENO
read -p
tap_like \
        "$REPLY" \
        '^[0-9:.]+ \tYou probably need {{define "callback"}} \.\.\. {{end}} around your template\.' \
        "Helpful message after template with outside-subtemplate data" \
        "$0" $LINENO
exec 9>&p; exec 9>&-; wait

# template_ok makes sure a template doesn't cause curlrevshell to print an
# error on startup.
#
# Arguments:
# $1 - Path to template
# $2 - Test name
# $3 - Lineno
function template_ok {
        gorun -template "$1" |&
        for i in `jot 3`; do read -p; done
        tap_like "$REPLY" "^[0-9:.]+ To get a shell:" "$2" "$0" "$3"
        exec 9>&p; exec 9>&-; wait
}

# Make sure we don't get a warning if we're not using a template.
template_ok "" "Default template parses ok" $LINENO

# Extract ALL the template.md templates!
./t/extract_templates.awk -v TMPLD="$TMPLD" doc/template.md
tap_ok $? "Awk exited with status 0" "$0" $LINENO # -e means $? will be 0

# Make sure we actually got templates.
tap_isnt "$(ls "$TMPLD")" "" "List extracted template files" "$0" $LINENO

# Make sure the templates parse ok.
for tmpl in $TMPLD/*; do
        template_ok $tmpl "doc/template.md $(basename "$tmpl") works" $LINENO
done

# vim: ft=sh
