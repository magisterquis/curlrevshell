package shellfuncsfile

/*
 * filter_perl.go
 * Turn a perl script into a shell function
 * By J. Stuart McMurray
 * Created 20240706
 * Last Modified 20241009
 */

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/magisterquis/curlrevshell/lib/uu"
)

const (
	singleQuote     = "'" /* Can't have this in our UUEncoded script. */
	safeSingleQuote = "s" /* Use this instead. */
	backslash       = `\` /* Can't have this either. */
	safeBackslash   = "b" /* Use this instead. */
)

// perlTemplate is what we use to convert uuencoded perl into a shell function.
var perlTemplate = template.Must(template.New("perl").Parse(`
{{- .LeadComments}}{{.FuncName}}() {(
(exit $((0 != $#))) || set -- -e "" -- "$@";
PERL5OPT=-d PERL5DB='BEGIN{eval(unpack(u,q{` + "`" + `
{{.PerlUU}}
}=~y/sb/\47\134/r));die"Error: $@"if(""ne$@);exit}' perl "$@"; )}` + "\n",
))

// FromPerl converts the Perl script read from r into a shell function.  If
// there is no perl, the function body is empty.
func FromPerl(name string, r io.Reader) ([]byte, error) {
	/* Work out our function's name. */
	funcName := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	/* Slurp and clean the script. */
	b, err := io.ReadAll(r)
	if nil != err {
		return nil, fmt.Errorf("slurping: %w", err)
	} else if 0 == len(b) {
		return []byte(fmt.Sprintf("%s() {}\n", funcName)), nil
	}
	leadComments, perl := cleanPerl(string(b))

	/* UUEncode, removing single quotes and trailing whitespace. */
	perlUU := string(uu.AppendEncode(nil, []byte(perl)))
	perlUU = strings.TrimSpace(perlUU)
	perlUU = strings.ReplaceAll(perlUU, singleQuote, safeSingleQuote)
	perlUU = strings.ReplaceAll(perlUU, backslash, safeBackslash)

	/* Write a shell function. */
	ret := new(bytes.Buffer)
	if err := perlTemplate.Execute(ret, map[string]string{
		"FuncName":     funcName,
		"LeadComments": leadComments,
		"PerlUU":       perlUU,
	}); nil != err {
		return nil, fmt.Errorf("rolling shell function: %w", err)
	}

	return ret.Bytes(), nil
}

// cleanPerl first trims leading and trailing whitespace with
// [strings.TrimSpace] and then replaces comments up to the first non-comment,
// non-blank line with newlines.
//
// Comments from the start of the file up to the first non-comment, non-blank
// line are returned in leadComments.  An initial line starting with #! or
// just # are discarded.
//
// The returned slices will both either be the empty string or end in a
// newline.
func cleanPerl(rawPerl string) (leadComments, perl string) {
	perl = strings.TrimSpace(rawPerl) /* Don't need too much whitespace. */
	if 0 == len(rawPerl) {
		return "", ""
	}

	/* We really operate more on lines than anything. */
	lines := strings.Split(perl, "\n")

	/* Split off the first comments. */
	var leadCommentLines []string
	for _, line := range lines {
		if !strings.HasPrefix(line, "#") {
			break
		}
		leadCommentLines = append(leadCommentLines, line)
	}
	start := 0
	for i, lcl := range leadCommentLines {
		if strings.HasPrefix(lcl, "#!") || "#" == lcl {
			start = i + 1
			continue
		}
		break
	}
	leadCommentLines = leadCommentLines[start:]

	/* Turn the lead comments into newlines. */
LOOP:
	for i, line := range lines {
		switch {
		case "\n" == line: /* Already a blank line. */
			continue
		case strings.HasPrefix(line, "#"): /* Comment. */
			lines[i] = "" /* Remove the comment. */
		default: /* First code line */
			break LOOP
		}
	}

	/* rejoin returns an empty string if ss has no strings, or joins ss
	with newlines and adds a trailing newline otherwise. */
	rejoin := func(ss []string) string {
		if 0 == len(ss) {
			return ""
		}
		return strings.Join(ss, "\n") + "\n"
	}

	return rejoin(leadCommentLines), rejoin(lines)
}
