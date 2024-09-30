package hsrv

/*
 * handlers_test.go
 * Tests for handlers.go
 * By J. Stuart McMurray
 * Created 20240324
 * Last Modified 20240930
 */

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/magisterquis/curlrevshell/internal/iobroker"
	"github.com/magisterquis/curlrevshell/lib/opshell"
)

func TestServerFileHandler(t *testing.T) {
	cl, _, och, s, _ := newTestServerMaybeWithDir(t, true)
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
		cl, _, och, s, _ := newTestServerMaybeWithDir(t, true)
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
	cl, ich, _, s, _ := newTestServer(t)

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
	cl, ich, och, s, shutdown := newTestServer(t)

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

		/* Wait for the request to finish and server to end. */
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
			Line: fmt.Sprintf(
				"[192.0.2.1] Input connected: ID %q",
				id,
			),
		}, {
			Color: ErrorColor,
			Line: "[192.0.2.1] Input connection closed: " +
				wantErr,
		}, {
			Color: ErrorColor,
			Line: "[192.0.2.1] " +
				iobroker.ShellDisconnectedMessage,
		}, {
			Color: ScriptColor,
			Line:  "To get a shell:",
		}, {
			Color:       ScriptColor,
			Line:        s.cbHelp,
			NoTimestamp: true,
		}}
		opshell.ExpectShellMessages(t, och, wantCLines...)

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

	/* Make sure we have no leftovers. */
	opshell.ExpectNoShellMessages(t, och, shutdown)
}

func TestServerInputHandler_RejectSecondConnection(t *testing.T) {
	cl, ich, och, s, shutdown := newTestServer(t)
	defer close(ich) /* Don't keep server hanging. */

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(errors.New("test finished"))

	/* First (connected) connection. */
	t.Run("connected_connection", func(t *testing.T) {
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
		wantCLine := []opshell.CLine{{
			Color: ConnectedColor,
			Line: fmt.Sprintf(
				"[192.0.2.1] Input connected: ID %q",
				id,
			),
		}}
		opshell.ExpectShellMessages(t, och, wantCLine...)
		cl.ExpectEmpty(
			t,
			`{"time":"","level":"INFO","msg":"New connection",`+
				`"http_request":{`+
				`"remote_addr":"192.0.2.1:1234",`+
				`"method":"GET","request_uri":"/i/`+id+`",`+
				`"protocol":"HTTP/1.1","host":"example.com",`+
				`"sni":"","user_agent":"","id":"`+id+`"},`+
				`"direction":"input"}`,
		)
	})

	/* Second (rejected) connection. */
	t.Run("rejected_connection", func(t *testing.T) {
		id := "moose"
		rr := httptest.NewRecorder()
		rr.Body = new(bytes.Buffer)
		req := httptest.NewRequest(http.MethodGet, "/i/"+id, nil)
		req.SetPathValue(idParam, id)
		s.inputHandler(rr, req)

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
		cl.ExpectEmpty(
			t,
			`{"time":"","level":"ERROR",`+
				`"msg":"Connection already established",`+
				`"http_request":{`+
				`"remote_addr":"192.0.2.1:1234",`+
				`"method":"GET","request_uri":"/i/`+id+`",`+
				`"protocol":"HTTP/1.1","host":"example.com",`+
				`"sni":"","user_agent":"","id":"`+id+`"},`+
				`"direction":"input"}`,
		)
		opshell.ExpectShellMessages(t, och, opshell.CLine{
			Color: ErrorColor,
			Line: fmt.Sprintf(
				"[192.0.2.1] Rejected unexpected input connection with ID %q",
				id,
			),
		})
	})

	/* Wait for the first connection to die. */
	wantErr := "test disconnect"
	cancel(errors.New(wantErr))
	wg.Wait()

	/* Make sure logs are good. */
	wantCLines := []opshell.CLine{{
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
	opshell.ExpectShellMessages(t, och, wantCLines...)
	cl.ExpectEmpty(
		t,
		`{"time":"","level":"ERROR","msg":"Disconnected",`+
			`"http_request":{"remote_addr":"192.0.2.1:1234",`+
			`"method":"GET","request_uri":"/i/kittens",`+
			`"protocol":"HTTP/1.1","host":"example.com","sni":"",`+
			`"user_agent":"","id":"kittens"},"direction":"input",`+
			`"error":"`+wantErr+`"}`,
	)
	opshell.ExpectNoShellMessages(t, och, shutdown)
}

func TestServerOutputHandler(t *testing.T) {
	cl, ich, och, s, shutdown := newTestServer(t)
	defer close(ich) /* Don't keep server hanging. */
	output := "moose"
	id := "kittens"

	/* Normal output. */
	t.Run("normal_output", func(t *testing.T) {
		/* Roll a request. */
		rr := httptest.NewRecorder()
		rr.Body = new(bytes.Buffer)
		pr, pw := io.Pipe()
		req := httptest.NewRequest(http.MethodGet, "/o/"+id, pr)
		req.SetPathValue(idParam, id)

		/* Send off the output but keep the request open. */
		ech := make(chan error)
		go func() {
			if _, err := pw.Write([]byte(output)); nil != err {
				ech <- fmt.Errorf("sending output: %w", err)
			}
			ech <- nil
		}()

		/* Connect to the output handler. */
		go func() {
			s.outputHandler(rr, req)

			/* Did it work? */
			if http.StatusOK != rr.Code {
				ech <- fmt.Errorf("Non-OK Code %d", rr.Code)
				return
			}
			if got := rr.Body.Len(); got != 0 {
				ech <- fmt.Errorf(
					"response body non-empty, "+
						"has %d bytes",
					got,
				)
			}
			ech <- nil
		}()

		/* Wait until our output gets there. */
		wantLogs := []opshell.CLine{{
			Color: ConnectedColor,
			Line: fmt.Sprintf(
				"[192.0.2.1] Output connected: ID %q",
				id,
			),
		}, {
			Line:  output,
			Plain: true,
		}}
		opshell.ExpectShellMessages(t, och, wantLogs...)

		/* Disconnect. */
		if err := pw.Close(); nil != err {
			t.Errorf("Error closing output pipe: %s", err)
		}
		for range 2 {
			err, ok := <-ech
			if !ok {
				t.Fatalf("Error channel unexpected closed")
			} else if nil != err {
				t.Errorf("Handle/Output error: %s", err)
			}
		}

		/* Make sure our shell restarts itself. */
		wantLogs = []opshell.CLine{{
			Color: ErrorColor,
			Line:  "[192.0.2.1] Output connection closed",
		}, {
			Color: ErrorColor,
			Line: "[192.0.2.1] " +
				iobroker.ShellDisconnectedMessage,
		}, {
			Color: ScriptColor,
			Line:  "To get a shell:",
		}, {
			Color:       ScriptColor,
			Line:        s.cbHelp,
			NoTimestamp: true,
		}}
		opshell.ExpectShellMessages(t, och, wantLogs...)

		/* And make sure logs look good. */
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
			Line: fmt.Sprintf(
				"[192.0.2.1] Input connected: ID %q",
				id,
			),
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
			Line: fmt.Sprintf(
				"[192.0.2.1] Rejected output "+
					"connection with ID %q, expected %q",
				newid,
				id,
			),
		}, {
			Color: ErrorColor,
			Line: "[192.0.2.1] Input connection closed: " +
				wantErr,
		}, {
			Color: ErrorColor,
			Line: "[192.0.2.1] " +
				iobroker.ShellDisconnectedMessage,
		}, {
			Color: ScriptColor,
			Line:  "To get a shell:",
		}, {
			Color:       ScriptColor,
			Line:        s.cbHelp,
			NoTimestamp: true,
		}}
		opshell.ExpectShellMessages(t, och, wantLogs...)
		cl.ExpectEmpty(
			t,
			`{"time":"","level":"INFO","msg":"New connection",`+
				`"http_request":{`+
				`"remote_addr":"192.0.2.1:1234",`+
				`"method":"GET","request_uri":"/i/kittens",`+
				`"protocol":"HTTP/1.1","host":"example.com",`+
				`"sni":"","user_agent":"","id":"kittens"},`+
				`"direction":"input"}`,
			`{"time":"","level":"ERROR","msg":"Incorrect key",`+
				`"http_request":{`+
				`"remote_addr":"192.0.2.1:1234",`+
				`"method":"GET","request_uri":"/o/kittens",`+
				`"protocol":"HTTP/1.1","host":"example.com",`+
				`"sni":"","user_agent":"","id":"zoomies!"},`+
				`"direction":"output","key":"kittens",`+
				`"incorrect_key":"zoomies!"}`,
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
	opshell.ExpectNoShellMessages(t, och, shutdown)
}

func TestServerOutputHandler_DisconnectInput(t *testing.T) {
	cl, ich, och, s, shutdown := newTestServer(t)
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
	opshell.ExpectShellMessages(t, och, opshell.CLine{
		Color: ConnectedColor,
		Line:  fmt.Sprintf("[192.0.2.1] Input connected: ID %q", id),
	})

	/* Hook up output and handler and make sure shell output gets there. */
	pr, pw := io.Pipe()
	defer pw.Close()
	defer pr.Close()
	orr := httptest.NewRecorder()
	orr.Body = new(bytes.Buffer)
	oreq := httptest.NewRequest(http.MethodGet, "/o/"+id, pr)
	oreq.SetPathValue(idParam, id)
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.outputHandler(orr, oreq)
	}()
	if _, err := pw.Write([]byte(output)); nil != err {
		t.Fatalf("Failed to send output: %s", err)
	}
	pw.Close()
	wantLogs := []opshell.CLine{{
		Color: ConnectedColor,
		Line:  fmt.Sprintf("[192.0.2.1] Output connected: ID %q", id),
	}, {
		Color: ConnectedColor,
		Line:  "[192.0.2.1] " + iobroker.ShellReadyMessage,
	}, {
		Line:  output,
		Plain: true,
	}}
	opshell.ExpectShellMessages(t, och, wantLogs...)

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
	wantLogs = []opshell.CLine{{
		Color: ErrorColor,
		Line:  "[192.0.2.1] Output connection closed",
	}, {
		Color: ErrorColor,
		Line:  "[192.0.2.1] Input connection closed",
	}, {
		Color: ErrorColor,
		Line:  "[192.0.2.1] " + iobroker.ShellDisconnectedMessage,
	}, {
		Color: ScriptColor,
		Line:  "To get a shell:",
	}, {
		Color:       ScriptColor,
		Line:        s.cbHelp,
		NoTimestamp: true,
	}}
	opshell.ExpectShellMessages(t, och, wantLogs...)
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
	opshell.ExpectNoShellMessages(t, och, shutdown)
}

func TestServerInOutHandler(t *testing.T) {
	cl, ich, och, s, shutdown := newTestServer(t)
	defer close(ich) /* Don't keep server hanging. */

	/* Hook up a connection. */
	pr, pw := io.Pipe()
	rr := httptest.NewRecorder()
	rr.Body = new(bytes.Buffer)
	req := httptest.NewRequest(
		http.MethodPost,
		"/io",
		pr,
	)

	/* Set the handler handling. */
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.inOutHandler(rr, req)
	}()

	/* Make sure we got a connection. */
	cl.ExpectUnordered(
		t,
		`{"time":"","level":"INFO","msg":"New connection",`+
			`"http_request":{"remote_addr":"192.0.2.1:1234",`+
			`"method":"POST","request_uri":"/io",`+
			`"protocol":"HTTP/1.1","host":"example.com","sni":"",`+
			`"user_agent":"","id":""},`+
			`"direction":"output"}`,
		`{"time":"","level":"INFO","msg":"New connection",`+
			`"http_request":{"remote_addr":"192.0.2.1:1234",`+
			`"method":"POST","request_uri":"/io",`+
			`"protocol":"HTTP/1.1","host":"example.com","sni":"",`+
			`"user_agent":"","id":""},`+
			`"direction":"input"}`,
	)
	opshell.ExpectShellMessages(t, och, opshell.CLine{
		Color: ConnectedColor,
		Line: fmt.Sprintf(
			"[192.0.2.1] %s",
			iobroker.ShellReadyMessage,
		),
	})
	if t.Failed() {
		t.FailNow()
	}

	/* Make sure we can send to the shell. */
	t.Run("input", func(t *testing.T) {
		have := "kittens"
		/* Send the input. */
		ich <- have
		/* Make sure we got it back and logged it properly. */
		want := have + "\n"
		wantJSON, err := json.Marshal(want)
		if nil != err {
			t.Fatalf("Error JSONifying %q: %s", want, err)
		}
		cl.Expect(
			t,
			`{"time":"","level":"INFO","msg":"Shell I/O",`+
				`"http_request":{`+
				`"remote_addr":"192.0.2.1:1234",`+
				`"method":"POST","request_uri":"/io",`+
				`"protocol":"HTTP/1.1","host":"example.com",`+
				`"sni":"","user_agent":"","id":""},`+
				`"direction":"input",`+
				`"data":`+string(wantJSON)+`}`,
		)
		if got := rr.Body.String(); got != want {
			t.Errorf(
				"Shell got wrong data:\n"+
					"have: %q\n"+
					" got: %q\n"+
					"want: %q",
				have,
				got,
				want,
			)
		}
	})

	/* Make sure we can receive from the shell. */
	t.Run("output", func(t *testing.T) {
		var (
			have = "kittens"
			werr error
			wg   sync.WaitGroup
		)
		/* Send the output. */
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, werr = pw.Write([]byte(have))
		}()
		/* Make sure we logged it and displayed it properly. */
		wantJSON, err := json.Marshal(have)
		if nil != err {
			t.Fatalf("Error JSONifying %q: %s", have, err)
		}
		cl.Expect(
			t,
			`{"time":"","level":"INFO","msg":"Shell I/O",`+
				`"http_request":{`+
				`"remote_addr":"192.0.2.1:1234",`+
				`"method":"POST","request_uri":"/io",`+
				`"protocol":"HTTP/1.1","host":"example.com",`+
				`"sni":"","user_agent":"","id":""},`+
				`"direction":"output",`+
				`"data":`+string(wantJSON)+`}`,
		)
		opshell.ExpectShellMessages(t, och, opshell.CLine{
			Line:  have,
			Plain: true,
		})
		/* Make sure our write actually worked. */
		wg.Wait()
		if nil != werr {
			t.Errorf("Error sending shell output: %s", err)
		}
	})

	/* Make sure we disconnect properly. */
	t.Run("disconnect", func(t *testing.T) {
		/* Kill our shell. */
		pw.Close()
		/* Make sure we got a connection. */
		cl.ExpectUnordered(
			t,
			`{"time":"","level":"INFO","msg":"Disconnected",`+
				`"http_request":{`+
				`"remote_addr":"192.0.2.1:1234",`+
				`"method":"POST","request_uri":"/io",`+
				`"protocol":"HTTP/1.1","host":"example.com",`+
				`"sni":"","user_agent":"","id":""},`+
				`"direction":"input"}`,
			`{"time":"","level":"INFO","msg":"Disconnected",`+
				`"http_request":{`+
				`"remote_addr":"192.0.2.1:1234",`+
				`"method":"POST","request_uri":"/io",`+
				`"protocol":"HTTP/1.1","host":"example.com",`+
				`"sni":"","user_agent":"","id":""},`+
				`"direction":"output"}`,
		)
		opshell.ExpectShellMessages(t, och, []opshell.CLine{{
			Color: ErrorColor,
			Line: fmt.Sprintf(
				"[192.0.2.1] Output side of bidirectional " +
					"connection closed",
			),
		}, {
			Color: ErrorColor,
			Line: fmt.Sprintf(
				"[192.0.2.1] Input side of bidirectional " +
					"connection closed",
			),
		}, {
			Color: ErrorColor,
			Line: fmt.Sprintf(
				"[192.0.2.1] %s",
				iobroker.ShellDisconnectedMessage,
			),
		}, {
			Color: ScriptColor,
			Line:  "To get a shell:",
		}, {
			Color:       ScriptColor,
			Line:        s.cbHelp,
			NoTimestamp: true,
		}}...)
		opshell.ExpectNoShellMessages(t, och, shutdown)
	})
}
