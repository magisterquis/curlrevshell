package shellfuncsfile

/*
 * filter_perl.go
 * Turn a perl script into a shell function
 * By J. Stuart McMurray
 * Created 20240706
 * Last Modified 20240707
 */

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
	"time"
)

// EnvVarPrefix is the prefix for the environment variable we use for passing
// perl code via an environment variable.  A timestamp will be appended.
const EnvVarPrefix = "SFF"

// testEnvVar is like the generated environment variable from EnvVarPrefix,
// but static when used in a test.
const testEnvVar = "SFF_1720354778930267614"

// perlTemplate is what we use to convert hexed perl into a shell function.
var perlTemplate = template.Must(template.New("perl").Parse(
	`{{.FuncName}}() { {{.EnvName}}={{.PerlHex}} ` +
		`perl -e '` +
		`eval(pack"H*",$ENV{ {{- .EnvName -}} });` +
		`die"Error: $@"if(""ne$@)' ` +
		`"$@"; }` + "\n",
))

// FromPerl converts the Perl script read from r into a shell function.
func FromPerl(name string, r io.Reader) ([]byte, error) {
	/* Slurp and clean the script. */
	perl, err := io.ReadAll(r)
	if nil != err {
		return nil, fmt.Errorf("slurping: %w", err)
	}

	/* Work out the environment variable. */
	var envVarName string
	if testing.Testing() {
		envVarName = testEnvVar
	} else {
		envVarName = fmt.Sprintf(
			"%s_%d",
			EnvVarPrefix,
			time.Now().UnixNano(),
		)
	}

	/* Write a shell function. */
	ret := new(bytes.Buffer)
	if err := perlTemplate.Execute(ret, map[string]string{
		"EnvName": envVarName,
		"FuncName": strings.TrimSuffix(
			filepath.Base(name), /* For just in case. */
			filepath.Ext(name),
		),
		"PerlHex": hex.EncodeToString(cleanPerl(perl)),
	}); nil != err {
		return nil, fmt.Errorf("rolling shell function: %w", err)
	}

	return ret.Bytes(), nil
}

// cleanPerl returns perl with leading comments, blank lines, and whitespace
// removed, and trailing whitespace removed.
func cleanPerl(perl []byte) []byte {
	perl = bytes.TrimSpace(perl) /* Cleans up the end. */
	/* Keep trying until we're not starting with whitespace or a
	comment. */
	for 0 != len(perl) {
		/* Not starting with whitespace anymore, anyways. */
		perl = bytes.TrimSpace(perl)
		if 0 == len(perl) {
			break
		}
		/* If the first line's not a comment, life's good. */
		if '#' != perl[0] {
			return perl
		}
		/* It's a comment, remove it. */
		if _, perl, _ = bytes.Cut(perl, []byte("\n")); nil == perl {
			perl = make([]byte, 0)
		}
	}
	return perl
}
