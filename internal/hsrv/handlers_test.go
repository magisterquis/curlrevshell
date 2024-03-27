package hsrv

/*
 * handlers_test.go
 * Tests for handlers.go
 * By J. Stuart McMurray
 * Created 20240324
 * Last Modified 20240327
 */

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/magisterquis/curlrevshell/lib/opshell"
)

func TestServerFileHandler(t *testing.T) {
	_, och, s := newTestServer(t)
	data := "kittens"
	fn := "fname"
	ffn := filepath.Join(s.fdir, fn)
	if err := os.WriteFile(ffn, []byte(data), 0600); nil != err {
		t.Fatalf("Error writing %s: %s", ffn, err)
	}

	/* Make sure directory listing works. */
	t.Run("directory_listing", func(t *testing.T) {
		rr := httptest.NewRecorder()
		rr.Body = new(bytes.Buffer)
		s.fileHandler(rr, httptest.NewRequest(
			http.MethodGet,
			"/",
			nil,
		))
		if http.StatusOK != rr.Code {
			t.Errorf("Non-OK Code %d", rr.Code)
		}
		want := `<pre>
<a href="fname">fname</a>
</pre>
`
		if got := rr.Body.String(); got != want {
			t.Errorf(
				"Incorrect body:\n"+
					"got:\n%s\n"+
					"want:\n%s\n",
				got,
				want,
			)
		}
		wantLog := opshell.CLine{
			Color: FileColor,
			Line:  "[192.0.2.1] File requested: /",
		}
		if got := <-och; got != wantLog {
			t.Errorf(
				"Incorrect log message:\n got: %#v\nwant: %#v",
				got,
				wantLog,
			)
		}
	})
	/* Make sure directory listing works. */
	t.Run("file_in_directory", func(t *testing.T) {
		rr := httptest.NewRecorder()
		rr.Body = new(bytes.Buffer)
		s.fileHandler(rr, httptest.NewRequest(
			http.MethodGet,
			"/"+fn,
			nil,
		))
		if http.StatusOK != rr.Code {
			t.Errorf("Non-OK Code %d", rr.Code)
		}
		want := data
		if got := rr.Body.String(); got != want {
			t.Errorf(
				"Incorrect body:\n"+
					"got:\n%s\n"+
					"want:\n%s\n",
				got,
				want,
			)
		}
		wantLog := opshell.CLine{
			Color: FileColor,
			Line:  "[192.0.2.1] File requested: /fname",
		}
		if got := <-och; got != wantLog {
			t.Errorf(
				"Incorrect log message:\n got: %#v\nwant: %#v",
				got,
				wantLog,
			)
		}
	})

	t.Run("file", func(t *testing.T) {
		_, och, s := newTestServer(t)
		s.fdir = ffn
		rr := httptest.NewRecorder()
		rr.Body = new(bytes.Buffer)
		s.fileHandler(rr, httptest.NewRequest(
			http.MethodGet,
			"/dummy",
			nil,
		))
		if http.StatusOK != rr.Code {
			t.Errorf("Non-OK Code %d", rr.Code)
		}
		want := data
		if got := rr.Body.String(); got != want {
			t.Errorf(
				"Incorrect body:\n"+
					"got:\n%s\n"+
					"want:\n%s\n",
				got,
				want,
			)
		}
		wantLog := opshell.CLine{
			Color: FileColor,
			Line:  "[192.0.2.1] File requested: /dummy",
		}
		if got := <-och; got != wantLog {
			t.Errorf(
				"Incorrect log message:\n got: %#v\nwant: %#v",
				got,
				wantLog,
			)
		}
	})
}

func TestServerInputHandler(t *testing.T) {
	ich, och, s := newTestServer(t)
	/* Input lines. */
	haveLines := []string{"kittens", "moose"}
	for _, l := range haveLines {
		ich <- l
	}
	close(ich)

	/* Make request as an implant. */
	id := "kittens"
	rr := httptest.NewRecorder()
	rr.Body = new(bytes.Buffer)
	req := httptest.NewRequest(http.MethodGet, "/i/"+id, nil)
	req.SetPathValue(idParam, id)
	s.inputHandler(rr, req)

	/* Did it work? */
	if http.StatusOK != rr.Code {
		t.Errorf("Non-OK Code %d", rr.Code)
	}
	wantBody := strings.Join(haveLines, "\n") + "\n"
	if got := rr.Body.String(); got != wantBody {
		t.Errorf(
			"Incorrect body:\n"+
				"got:\n%s\n"+
				"want:\n%s\n",
			got,
			wantBody,
		)
	}

	/* Make sure logs are good. */
	wantLogs := []opshell.CLine{{
		Color: ConnectedColor,
		Line:  "[192.0.2.1] Got a shell: ID:kittens",
	}}
	wantN := len(wantLogs)
	gotN := len(och)
	if gotN != wantN {
		t.Errorf("Expected %d logs, got %d", wantN, gotN)
	}
	for i := 0; i < min(gotN, wantN); i++ {
		if got := <-och; got != wantLogs[i] {
			t.Errorf(
				"Incorrect log message:\n got: %#v\nwant: %#v",
				got,
				wantLogs[i],
			)
		}
	}

	/* Make sure another implant can connect. */
	rr = httptest.NewRecorder()
	rr.Body = new(bytes.Buffer)
	req = httptest.NewRequest(http.MethodGet, "/i/"+id, nil)
	id = "moose"
	req.SetPathValue(idParam, id)
	s.inputHandler(rr, req)

	/* Did it work? */
	if http.StatusOK != rr.Code {
		t.Errorf("Non-OK Code on reconnect: %d", rr.Code)
	}
	if got := rr.Body.Len(); got != 0 {
		t.Errorf(
			"Second request respnose body non-empty, has %d bytes",
			got,
		)
	}

	/* Log correct? */
	wantLogs = []opshell.CLine{{
		Color: ConnectedColor,
		Line:  "[192.0.2.1] Got a shell: ID:moose",
	}}
	wantN = len(wantLogs)
	gotN = len(och)
	if gotN != wantN {
		t.Errorf(
			"Expected %d logs after second request, got %d",
			wantN,
			gotN,
		)
	}
	for i := 0; i < min(gotN, wantN); i++ {
		if got := <-och; got != wantLogs[i] {
			t.Errorf(
				"Incorrect log message:\n got: %#v\nwant: %#v",
				got,
				wantLogs[i],
			)
		}
	}
}

func TestServerInputHandler_RejectSecondConnection(t *testing.T) {
	ich, och, s := newTestServer(t)
	defer close(ich) /* Don't keep server hanging. */

	/* First (connected) connection. */
	id := "kittens"
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/i/"+id, nil)
	req.SetPathValue(idParam, id)
	go s.inputHandler(rr, req)
	wantLog := opshell.CLine{
		Color: ConnectedColor,
		Line:  "[192.0.2.1] Got a shell: ID:kittens",
	}
	if gotLog := <-och; gotLog != wantLog {
		t.Fatalf(
			"Incorrect log from first connection:\n"+
				" got: %#v\n"+
				"want: %#v",
			gotLog,
			wantLog,
		)
	}

	/* Second (rejected) connection. */
	id = "moose"
	rr = httptest.NewRecorder()
	rr.Body = new(bytes.Buffer)
	req = httptest.NewRequest(http.MethodGet, "/i/"+id, nil)
	req.SetPathValue(idParam, id)
	s.inputHandler(rr, req)

	/* Did it work? */
	if want := http.StatusServiceUnavailable; want != rr.Code {
		t.Errorf(
			"Incorrect status code\n"+
				" got: %d\n"+
				"want: %d",
			rr.Code,
			want,
		)
	}
	wantBody := MultiConnectionMessage + "\n"
	if got := rr.Body.String(); got != wantBody {
		t.Errorf(
			"Incorrect body:\n"+
				" got:\n%s\n"+
				"want:\n%s\n",
			got,
			wantBody,
		)
	}

	/* Make sure logs are good. */
	wantLogs := []opshell.CLine{{
		Color: ErrorColor,
		Line:  "[192.0.2.1] Rejected connection from ID moose, current ID is kittens",
	}}
	wantN := len(wantLogs)
	gotN := len(och)
	if gotN != wantN {
		t.Errorf("Expected %d logs, got %d", wantN, gotN)
	}
	for i := 0; i < min(gotN, wantN); i++ {
		if got := <-och; got != wantLogs[i] {
			t.Errorf(
				"Incorrect log message:\n got: %#v\nwant: %#v",
				got,
				wantLogs[i],
			)
		}
	}
}

func TestServerOutputHandler(t *testing.T) {
	ich, och, s := newTestServer(t)
	defer close(ich) /* Don't keep server hanging. */
	output := "moose"
	id := "kittens"

	/* Try with nothing connected. */
	t.Run("nothing_connected", func(t *testing.T) {
		rr := httptest.NewRecorder()
		rr.Body = new(bytes.Buffer)
		req := httptest.NewRequest(
			http.MethodGet,
			"/o/"+id,
			strings.NewReader(output),
		)
		req.SetPathValue(idParam, id)
		s.outputHandler(rr, req)
		/* Did it work? */
		if want := http.StatusFailedDependency; want != rr.Code {
			t.Fatalf(
				"Incorrect status code\n"+
					" got: %d\n"+
					"want: %d",
				rr.Code,
				want,
			)
		}
		wantBody := UnexpectedOutputMessage + "\n"
		if got := rr.Body.String(); got != wantBody {
			t.Errorf(
				"Incorrect body:\n"+
					"got:\n%s\n"+
					"want:\n%s\n",
				got,
				wantBody,
			)
		}
		wantLog := opshell.CLine{
			Color: ErrorColor,
			Line: "[192.0.2.1] No connection but got " +
				"output from ID kittens",
		}
		if gotLog := <-och; gotLog != wantLog {
			t.Fatalf(
				"Incorrect log from input connection:\n"+
					" got: %#v\n"+
					"want: %#v",
				gotLog,
				wantLog,
			)
		}
	})

	/* Connect a connection. */
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/i/"+id, nil)
	req.SetPathValue(idParam, id)
	go s.inputHandler(rr, req)
	wantLog := opshell.CLine{
		Color: ConnectedColor,
		Line:  "[192.0.2.1] Got a shell: ID:kittens",
	}
	if gotLog := <-och; gotLog != wantLog {
		t.Fatalf(
			"Incorrect log from input connection:\n"+
				" got: %#v\n"+
				"want: %#v",
			gotLog,
			wantLog,
		)
	}

	/* Normal output. */
	t.Run("normal_output", func(t *testing.T) {
		rr := httptest.NewRecorder()
		rr.Body = new(bytes.Buffer)
		req := httptest.NewRequest(
			http.MethodGet,
			"/o/"+id,
			strings.NewReader(output),
		)
		req.SetPathValue(idParam, id)
		s.outputHandler(rr, req)
		/* Did it work? */
		if http.StatusOK != rr.Code {
			t.Errorf("Non-OK Code %d", rr.Code)
		}
		if got := rr.Body.Len(); got != 0 {
			t.Errorf("Response body non-empty, has %d bytes", got)
		}
		wantLogs := []opshell.CLine{{
			Line: output,
		}}
		wantN := len(wantLogs)
		gotN := len(och)
		if gotN != wantN {
			t.Errorf("Expected %d logs, got %d", wantN, gotN)
		}
		for i := 0; i < min(gotN, wantN); i++ {
			if got := <-och; got != wantLogs[i] {
				t.Errorf(
					"Incorrect log message:\n"+
						"got: %#v\n"+
						"want: %#v",
					got,
					wantLogs[i],
				)
			}
		}
	})

	/* Output for wrong ID. */
	t.Run("wrong_id", func(t *testing.T) {
		id := "moose"
		rr := httptest.NewRecorder()
		rr.Body = new(bytes.Buffer)
		req := httptest.NewRequest(
			http.MethodGet,
			"/o/"+id,
			strings.NewReader(output),
		)
		req.SetPathValue(idParam, id)
		s.outputHandler(rr, req)
		/* Did it work? */
		if want := http.StatusFailedDependency; want != rr.Code {
			t.Fatalf(
				"Incorrect status code\n"+
					" got: %d\n"+
					"want: %d",
				rr.Code,
				want,
			)
		}
		wantBody := UnexpectedOutputMessage + "\n"
		if got := rr.Body.String(); got != wantBody {
			t.Errorf(
				"Incorrect body:\n"+
					" got:\n%s\n"+
					"want:\n%s\n",
				got,
				wantBody,
			)
		}
		wantLogs := []opshell.CLine{{
			Color: ErrorColor,
			Line: "[192.0.2.1] Got output from ID moose " +
				"while ID kittens is connected",
		}}
		wantN := len(wantLogs)
		gotN := len(och)
		if gotN != wantN {
			t.Errorf("Expected %d logs, got %d", wantN, gotN)
		}
		for i := 0; i < min(gotN, wantN); i++ {
			if got := <-och; got != wantLogs[i] {
				t.Errorf(
					"Incorrect log message:\n"+
						" got: %#v\n"+
						"want: %#v",
					got,
					wantLogs[i],
				)
			}
		}
	})
}
