package hsrv

/*
 * handlers_test.go
 * Tests for handlers.go
 * By J. Stuart McMurray
 * Created 20240324
 * Last Modified 20240729
 */

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/magisterquis/curlrevshell/lib/opshell"
)

func TestServerFileHandler(t *testing.T) {
	cl, _, och, s := newTestServer(t)
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
		want := `<!doctype html>
<meta name="viewport" content="width=device-width">
<pre>
<a href="` + fn + `">` + fn + `</a>
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
		cl.ExpectEmpty(
			t,
			`{"time":"","level":"INFO","msg":"File requested",`+
				`"http_request":{`+
				`"remote_addr":"192.0.2.1:1234",`+
				`"method":"GET","request_uri":"/",`+
				`"protocol":"HTTP/1.1","host":"example.com",`+
				`"sni":"","user_agent":"","id":""},`+
				`"static_files_dir":"`+s.fdir+`"}`,
		)
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
			Line:  "[192.0.2.1] File requested: /" + fn,
		}
		if got := <-och; got != wantLog {
			t.Errorf(
				"Incorrect log message:\n got: %#v\nwant: %#v",
				got,
				wantLog,
			)
		}
		cl.ExpectEmpty(
			t,
			`{"time":"","level":"INFO","msg":"File requested",`+
				`"http_request":{`+
				`"remote_addr":"192.0.2.1:1234",`+
				`"method":"GET","request_uri":"/`+fn+`",`+
				`"protocol":"HTTP/1.1","host":"example.com",`+
				`"sni":"","user_agent":"","id":""},`+
				`"static_files_dir":"`+s.fdir+`"}`,
		)
	})

	t.Run("file", func(t *testing.T) {
		cl, _, och, s := newTestServer(t)
		s.fdir = ffn
		rr := httptest.NewRecorder()
		rr.Body = new(bytes.Buffer)
		dfn := "dummy"
		s.fileHandler(rr, httptest.NewRequest(
			http.MethodGet,
			"/"+dfn,
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
			Line:  "[192.0.2.1] File requested: /" + dfn,
		}
		if got := <-och; got != wantLog {
			t.Errorf(
				"Incorrect log message:\n got: %#v\nwant: %#v",
				got,
				wantLog,
			)
		}
		cl.ExpectEmpty(
			t,
			`{"time":"","level":"INFO","msg":"File requested",`+
				`"http_request":{`+
				`"remote_addr":"192.0.2.1:1234",`+
				`"method":"GET","request_uri":"/`+dfn+`",`+
				`"protocol":"HTTP/1.1","host":"example.com",`+
				`"sni":"","user_agent":"","id":""},`+
				`"static_files_dir":"`+s.fdir+`"}`,
		)
	})
	cl.ExpectEmpty(t)
}

/* Make sure closing stdin stops the handler. */
func TestServerInputHandler_CloseStdin(t *testing.T) {
	cl, ich, _, s := newTestServer(t)

	/* Roll a request */
	id := t.Name()
	rr := httptest.NewRecorder()
	rr.Body = new(bytes.Buffer)
	req := httptest.NewRequest(
		http.MethodGet,
		"/i/"+id,
		nil,
	)
	req.SetPathValue(idParam, id)

	/* Start the handler handling. */
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.inputHandler(rr, req)
	}()

	/* Close the input and wait for the handler to return. */
	close(ich)
	wg.Wait()

	/* Make sure we log the disconnect. */
	cl.ExpectEmpty(t,
		`{"time":"","level":"INFO","msg":"New connection",`+
			`"http_request":{`+
			`"remote_addr":"192.0.2.1:1234",`+
			`"method":"GET",`+
			`"request_uri":"/i/`+id+`",`+
			`"protocol":"HTTP/1.1","host":"example.com",`+
			`"sni":"","user_agent":"",`+
			`"id":"`+id+`"},"direction":"input"}`,
		`{"time":"","level":"INFO",`+
			`"msg":"Disconnected","http_request":{`+
			`"remote_addr":"192.0.2.1:1234","method":"GET",`+
			`"request_uri":"/i/`+id+`","protocol":"HTTP/1.1",`+
			`"host":"example.com","sni":"","user_agent":"",`+
			`"id":"`+id+`"},"direction":"input"}`,
	)
}

func TestServerInputHandler(t *testing.T) {
	cl, ich, och, s := newTestServer(t)

	try := func(t *testing.T) {
		/* Input lines. */
		haveLines := []string{t.Name() + "line1", t.Name() + "line2"}
		for _, l := range haveLines {
			ich <- l
		}

		/* Make request as an implant. */
		id := t.Name()
		rr := httptest.NewRecorder()
		rr.Body = new(bytes.Buffer)
		ctx, cancel := context.WithCancelCause(context.Background())
		defer cancel(errors.New("test finished"))
		req := httptest.NewRequest(
			http.MethodGet,
			"/i/"+id,
			nil,
		).WithContext(ctx)
		req.SetPathValue(idParam, id)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.inputHandler(rr, req)
		}()

		/* Should get a server message plus two input lines. */
		wantErr := "test disconnect"
		wantLogs := []string{
			`{"time":"","level":"INFO","msg":"New connection",` +
				`"http_request":{` +
				`"remote_addr":"192.0.2.1:1234",` +
				`"method":"GET",` +
				`"request_uri":"/i/` + id + `",` +
				`"protocol":"HTTP/1.1","host":"example.com",` +
				`"sni":"","user_agent":"",` +
				`"id":"` + id + `"},"direction":"input"}`,
		}
		for _, l := range haveLines {
			msg := `{"time":"","level":"INFO",` +
				`"msg":"Shell I/O","http_request":{` +
				`"remote_addr":"192.0.2.1:1234",` +
				`"method":"GET",` +
				`"request_uri":"/i/` + id + `",` +
				`"protocol":"HTTP/1.1","host":"example.com",` +
				`"sni":"","user_agent":"","id":"` + id + `"},` +
				`"direction":"input","data":"` + l + `\n"}`
			wantLogs = append(wantLogs, msg)
		}
		cl.ExpectEmpty(t, wantLogs...)

		/* Wait for the request to finish. */
		cancel(errors.New(wantErr))
		wg.Wait()

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

		/* Make sure shell output is good. */
		wantCLines := []opshell.CLine{{
			Color: ConnectedColor,
			Line:  "[192.0.2.1] Input connected: ID:" + id,
		}, {
			Color: ErrorColor,
			Line: "[192.0.2.1] Input connection closed: " +
				wantErr,
		}, {
			Color: ErrorColor,
			Line:  "[192.0.2.1] " + ShellDisconnectedMessage,
		}, {
			Color: ScriptColor,
			Line:  "To get a shell:",
		}, {
			Color:       ScriptColor,
			Line:        s.cbHelp,
			NoTimestamp: true,
		}}
		wantN := len(wantCLines)
		gotN := len(och)
		if gotN != wantN {
			t.Errorf(
				"Expected %d shell messages, got %d",
				wantN,
				gotN,
			)
		}
		for i := 0; i < min(gotN, wantN); i++ {
			if got := <-och; got != wantCLines[i] {
				t.Errorf(
					"Incorrect shell message:\n"+
						" got: %#v\n"+
						"want: %#v",
					got,
					wantCLines[i],
				)
			}
		}
		for 0 != len(och) {
			t.Errorf("Extra shell message: %#v", <-och)
		}
		/* Make sure we log the disconnect. */
		cl.ExpectEmpty(t, `{"time":"","level":"ERROR",`+
			`"msg":"Disconnected","http_request":{`+
			`"remote_addr":"192.0.2.1:1234","method":"GET",`+
			`"request_uri":"/i/`+id+`","protocol":"HTTP/1.1",`+
			`"host":"example.com","sni":"","user_agent":"",`+
			`"id":"`+id+`"},"direction":"input",`+
			`"error":"`+wantErr+`"}`,
		)
	}

	/* Try a couple of times, to make sure multiple implants work. */
	t.Run("kittens", try)
	t.Run("moose", try)

}

func TestServerInputHandler_RejectSecondConnection(t *testing.T) {
	cl, ich, och, s := newTestServer(t)
	defer close(ich) /* Don't keep server hanging. */

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(errors.New("test finished"))

	/* First (connected) connection. */
	id := "kittens"
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodGet,
		"/i/"+id,
		nil,
	).WithContext(ctx)
	req.SetPathValue(idParam, id)
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.inputHandler(rr, req)
	}()
	wantCLine := opshell.CLine{
		Color: ConnectedColor,
		Line:  "[192.0.2.1] Input connected: ID:kittens",
	}
	if gotCLine := <-och; gotCLine != wantCLine {
		t.Fatalf(
			"Incorrect terminal message from first connection:\n"+
				" got: %#v\n"+
				"want: %#v",
			gotCLine,
			wantCLine,
		)
	}
	cl.ExpectEmpty(
		t,
		`{"time":"","level":"INFO","msg":"New connection",`+
			`"http_request":{"remote_addr":"192.0.2.1:1234",`+
			`"method":"GET","request_uri":"/i/`+id+`",`+
			`"protocol":"HTTP/1.1","host":"example.com","sni":"",`+
			`"user_agent":"","id":"`+id+`"},"direction":"input"}`,
	)

	/* Second (rejected) connection. */
	id = "moose"
	rr = httptest.NewRecorder()
	rr.Body = new(bytes.Buffer)
	req2 := httptest.NewRequest(http.MethodGet, "/i/"+id, nil)
	req2.SetPathValue(idParam, id)
	s.inputHandler(rr, req2)

	/* Did it work? */
	if want := http.StatusOK; want != rr.Code {
		t.Errorf(
			"Incorrect status code\n"+
				" got: %d\n"+
				"want: %d",
			rr.Code,
			want,
		)
	}
	if got := len(rr.Body.String()); 0 != got {
		t.Errorf("Response body non-empty, has %d bytes", got)
	}

	/* Wait for the first connection to die. */
	wantErr := "test disconnect"
	cancel(errors.New(wantErr))
	wg.Wait()

	/* Make sure logs are good. */
	wantCLines := []opshell.CLine{{
		Color: ErrorColor,
		Line: "[192.0.2.1] Rejected unexpected input " +
			"connection with ID " + id,
	}, {
		Color: ErrorColor,
		Line:  "[192.0.2.1] Input connection closed: " + wantErr,
	}, {
		Color: ErrorColor,
		Line:  "[192.0.2.1] Shell is gone :(",
	}, {
		Color: ScriptColor,
		Line:  "To get a shell:",
	}, {
		Color:       ScriptColor,
		Line:        s.cbHelp,
		NoTimestamp: true,
	}}
	wantN := len(wantCLines)
	gotN := len(och)
	if gotN != wantN {
		t.Errorf("Expected %d shell messages, got %d", wantN, gotN)
	}
	for i := 0; i < min(gotN, wantN); i++ {
		if got := <-och; got != wantCLines[i] {
			t.Errorf(
				"Incorrect shell message:\n got: %#v\nwant: %#v",
				got,
				wantCLines[i],
			)
		}
	}
	for 0 != len(och) {
		t.Errorf("Unexpected shell message: %#v", <-och)
	}
	cl.ExpectEmpty(
		t,
		`{"time":"","level":"ERROR","msg":"Disconnected",`+
			`"http_request":{"remote_addr":"192.0.2.1:1234",`+
			`"method":"GET","request_uri":"/i/kittens",`+
			`"protocol":"HTTP/1.1","host":"example.com","sni":"",`+
			`"user_agent":"","id":"kittens"},"direction":"input",`+
			`"error":"`+wantErr+`"}`,
	)
}

func TestServerOutputHandler(t *testing.T) {
	cl, ich, och, s := newTestServer(t)
	defer close(ich) /* Don't keep server hanging. */
	output := "moose"
	id := "kittens"

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
			Color: ConnectedColor,
			Line:  "[192.0.2.1] Output connected: ID:kittens",
		}, {
			Line:  output,
			Plain: true,
		}, {
			Color: ErrorColor,
			Line:  "[192.0.2.1] Output connection closed",
		}, {
			Color: ErrorColor,
			Line:  "[192.0.2.1] " + ShellDisconnectedMessage,
		}, {
			Color: ScriptColor,
			Line:  "To get a shell:",
		}, {
			Color:       ScriptColor,
			Line:        s.cbHelp,
			NoTimestamp: true,
		}}
		wantN := len(wantLogs)
		gotN := len(och)
		if gotN != wantN {
			t.Errorf(
				"Expected %d shell messages, got %d",
				wantN,
				gotN,
			)
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
		for 0 != len(och) {
			t.Errorf("Extra shell message: %#v", <-och)
		}
		cl.ExpectEmpty(
			t,
			`{"time":"","level":"INFO","msg":"New connection",`+
				`"http_request":{`+
				`"remote_addr":"192.0.2.1:1234",`+
				`"method":"GET","request_uri":"/o/`+id+`",`+
				`"protocol":"HTTP/1.1","host":"example.com",`+
				`"sni":"","user_agent":"","id":"`+id+`"},`+
				`"direction":"output"}`,
			`{"time":"","level":"INFO","msg":"Shell I/O",`+
				`"http_request":{`+
				`"remote_addr":"192.0.2.1:1234",`+
				`"method":"GET","request_uri":"/o/`+id+`",`+
				`"protocol":"HTTP/1.1","host":"example.com",`+
				`"sni":"","user_agent":"",`+
				`"id":"`+id+`"},"direction":"output",`+
				`"data":"`+output+`"}`,
			`{"time":"","level":"INFO","msg":"Disconnected",`+
				`"http_request":{`+
				`"remote_addr":"192.0.2.1:1234",`+
				`"method":"GET","request_uri":"/o/`+id+`",`+
				`"protocol":"HTTP/1.1","host":"example.com",`+
				`"sni":"","user_agent":"","id":"`+id+`"},`+
				`"direction":"output"}`,
		)
	})

	/* Output for wrong ID. */
	t.Run("wrong_id", func(t *testing.T) {
		/* Make input request as an implant. */
		irr := httptest.NewRecorder()
		irr.Body = new(bytes.Buffer)
		ctx, cancel := context.WithCancelCause(context.Background())
		defer cancel(errors.New("test finished"))
		ireq := httptest.NewRequest(
			http.MethodGet,
			"/i/"+id,
			nil,
		).WithContext(ctx)
		ireq.SetPathValue(idParam, id)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.inputHandler(irr, ireq)
		}()

		/* Wait for the input to connect. */
		wantCLine := opshell.CLine{
			Color: ConnectedColor,
			Line:  "[192.0.2.1] Input connected: ID:kittens",
		}
		if got := <-och; wantCLine != got {
			t.Fatalf(
				"Incorrect input connected message:\n"+
					" got: %#v\n"+
					"want: %#v",
				got,
				wantCLine,
			)
		}

		newid := "zoomies!"
		rr := httptest.NewRecorder()
		rr.Body = new(bytes.Buffer)
		req := httptest.NewRequest(
			http.MethodGet,
			"/o/"+id,
			strings.NewReader(output),
		)
		req.SetPathValue(idParam, newid)
		s.outputHandler(rr, req)

		/* Wait for input handler to finish. */
		wantErr := "test disconnect"
		cancel(errors.New(wantErr))
		wg.Wait()

		/* Did it work? */
		if want := http.StatusOK; want != rr.Code {
			t.Fatalf(
				"Incorrect status code\n"+
					" got: %d\n"+
					"want: %d",
				rr.Code,
				want,
			)
		}
		if got := len(rr.Body.String()); 0 != got {
			t.Errorf("Response body non-empty, has %d bytes", got)
		}
		wantLogs := []opshell.CLine{{
			Color: ErrorColor,
			Line: "[192.0.2.1] Rejected output " +
				"connection with ID " + newid +
				", expected " + id,
		}, {
			Color: ErrorColor,
			Line:  "[192.0.2.1] Input connection closed: " + wantErr,
		}, {
			Color: ErrorColor,
			Line:  "[192.0.2.1] " + ShellDisconnectedMessage,
		}, {
			Color: ScriptColor,
			Line:  "To get a shell:",
		}, {
			Color:       ScriptColor,
			Line:        s.cbHelp,
			NoTimestamp: true,
		}}
		wantN := len(wantLogs)
		gotN := len(och)
		if gotN != wantN {
			t.Errorf(
				"Expected %d shell messages, got %d",
				wantN,
				gotN,
			)
		}
		for i := 0; i < min(gotN, wantN); i++ {
			if got := <-och; got != wantLogs[i] {
				t.Errorf(
					"Incorrect shell message:\n"+
						" got: %#v\n"+
						"want: %#v",
					got,
					wantLogs[i],
				)
			}
		}
		for 0 != len(och) {
			t.Errorf("Unexpected shell message: %#v", <-och)
		}
		cl.ExpectEmpty(
			t,
			`{"time":"","level":"INFO","msg":"New connection",`+
				`"http_request":{`+
				`"remote_addr":"192.0.2.1:1234",`+
				`"method":"GET","request_uri":"/i/kittens",`+
				`"protocol":"HTTP/1.1","host":"example.com",`+
				`"sni":"","user_agent":"","id":"kittens"},`+
				`"direction":"input"}`,
			`{"time":"","level":"ERROR","msg":"Disconnected",`+
				`"http_request":{`+
				`"remote_addr":"192.0.2.1:1234",`+
				`"method":"GET","request_uri":"/i/kittens",`+
				`"protocol":"HTTP/1.1","host":"example.com",`+
				`"sni":"","user_agent":"","id":"kittens"},`+
				`"direction":"input",`+
				`"error":"test disconnect"}`,
		)
	})

	/* Server logs? */
	cl.ExpectEmpty(t)
}

func TestServerOutputHandler_DisconnectInput(t *testing.T) {
	cl, ich, och, s := newTestServer(t)
	defer close(ich) /* Don't keep server hanging. */
	id := "kittens"
	output := "moose"

	/* Make input request as an implant. */
	irr := httptest.NewRecorder()
	irr.Body = new(bytes.Buffer)
	ireq := httptest.NewRequest(
		http.MethodGet,
		"/i/"+id,
		nil,
	)
	ireq.SetPathValue(idParam, id)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.inputHandler(irr, ireq)
	}()

	/* Wait for the input to connect. */
	wantCLine := opshell.CLine{
		Color: ConnectedColor,
		Line:  "[192.0.2.1] Input connected: ID:kittens",
	}
	if got := <-och; wantCLine != got {
		t.Fatalf(
			"Incorrect input connected message:\n"+
				" got: %#v\n"+
				"want: %#v",
			got,
			wantCLine,
		)
	}

	/* Send output, which starts and ends a shell. */
	orr := httptest.NewRecorder()
	orr.Body = new(bytes.Buffer)
	oreq := httptest.NewRequest(
		http.MethodGet,
		"/o/"+id,
		strings.NewReader(output),
	)
	oreq.SetPathValue(idParam, id)
	s.outputHandler(orr, oreq)

	/* Wait for input handler to finish as well. */
	wg.Wait()

	/* Did it work? */
	if http.StatusOK != irr.Code {
		t.Errorf("Non-OK Input Code %d", irr.Code)
	}
	if http.StatusOK != orr.Code {
		t.Errorf("Non-OK Output Code %d", orr.Code)
	}
	if got := irr.Body.Len(); got != 0 {
		t.Errorf("Input response body non-empty, has %d bytes", got)
	}
	if got := orr.Body.Len(); got != 0 {
		t.Errorf("Output response body non-empty, has %d bytes", got)
	}
	wantLogs := []opshell.CLine{{
		Color: ConnectedColor,
		Line:  "[192.0.2.1] Output connected: ID:kittens",
	}, {
		Color: ConnectedColor,
		Line:  "[192.0.2.1] " + ShellReadyMessage,
	}, {
		Line:  output,
		Plain: true,
	}, {
		Color: ErrorColor,
		Line:  "[192.0.2.1] Output connection closed",
	}, {
		Color: ErrorColor,
		Line:  "[192.0.2.1] Input connection closed",
	}, {
		Color: ErrorColor,
		Line:  "[192.0.2.1] " + ShellDisconnectedMessage,
	}, {
		Color: ScriptColor,
		Line:  "To get a shell:",
	}, {
		Color:       ScriptColor,
		Line:        s.cbHelp,
		NoTimestamp: true,
	}}
	wantN := len(wantLogs)
	gotN := len(och)
	if gotN != wantN {
		t.Errorf(
			"Expected %d shell messages, got %d",
			wantN,
			gotN,
		)
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
	for 0 != len(och) {
		t.Errorf("Extra shell message: %#v", <-och)
	}
	cl.ExpectEmpty(
		t,
		`{"time":"","level":"INFO","msg":"New connection",`+
			`"http_request":{"remote_addr":"192.0.2.1:1234",`+
			`"method":"GET","request_uri":"/i/kittens",`+
			`"protocol":"HTTP/1.1","host":"example.com","sni":"",`+
			`"user_agent":"","id":"kittens"},"direction":"input"}`,
		`{"time":"","level":"INFO","msg":"New connection",`+
			`"http_request":{"remote_addr":"192.0.2.1:1234",`+
			`"method":"GET","request_uri":"/o/kittens",`+
			`"protocol":"HTTP/1.1","host":"example.com","sni":"",`+
			`"user_agent":"","id":"kittens"},"direction":"output"}`,
		`{"time":"","level":"INFO","msg":"Shell I/O",`+
			`"http_request":{"remote_addr":"192.0.2.1:1234",`+
			`"method":"GET","request_uri":"/o/kittens",`+
			`"protocol":"HTTP/1.1","host":"example.com","sni":"",`+
			`"user_agent":"","id":"kittens"},`+
			`"direction":"output","data":"moose"}`,
		`{"time":"","level":"INFO","msg":"Disconnected",`+
			`"http_request":{"remote_addr":"192.0.2.1:1234",`+
			`"method":"GET","request_uri":"/o/kittens",`+
			`"protocol":"HTTP/1.1","host":"example.com","sni":"",`+
			`"user_agent":"","id":"kittens"},"direction":"output"}`,
		`{"time":"","level":"INFO","msg":"Disconnected",`+
			`"http_request":{"remote_addr":"192.0.2.1:1234",`+
			`"method":"GET","request_uri":"/i/kittens",`+
			`"protocol":"HTTP/1.1","host":"example.com","sni":"",`+
			`"user_agent":"","id":"kittens"},"direction":"input"}`,
	)
}
