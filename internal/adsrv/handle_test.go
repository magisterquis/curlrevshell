package adsrv

/*
 * handle_test.go
 * Tests for handle.go
 * By J. Stuart McMurray
 * Created 20260808
 * Last Modified 20260814
 */

import (
	"fmt"
	"io"
	"sync"
	"syscall"
	"testing"
	"testing/synctest"

	"github.com/magisterquis/curlrevshell/internal/bidirpipe"
	"github.com/magisterquis/curlrevshell/lib/crsadapter"
	"github.com/magisterquis/curlrevshell/lib/tlog"
)

// newTestJSONStreamPair returns two connected jsonstreams.
func newTestJSONStreamPair(t *testing.T) (a, b *crsadapter.Stream) {
	l, r := bidirpipe.New()
	t.Cleanup(func() { l.Close(); r.Close() })
	return crsadapter.NewStream(l), crsadapter.NewStream(r)
}

// Can we send ConnResponses?
func TestSendConnResponse(t *testing.T) {
	// Try tries to send rErr as a ConnResponse.
	try := func(t *testing.T, rErr error) {
		var (
			want   string
			c, s   = newTestJSONStreamPair(t)
			cr     crsadapter.ConnResponse
			tb, sl = tlog.NewBuffer()
			wg     sync.WaitGroup
		)
		/* Send the response. */
		wg.Go(func() {
			if !sendConnResponse(sl, s, rErr) {
				t.Errorf("sendConnResponse returned false")
			}
		})
		/* Come through correctly? */
		wg.Go(func() {
			/* Grab the response. */
			if err := c.DecodeNext(&cr); nil != err {
				t.Errorf("Decode error: %v", err)
				return
			}
			/* Correct? */
			if nil != rErr {
				want = rErr.Error()
			}
			if got := cr.Error; got != want {
				t.Errorf(
					"ConnResponse Error incorrect\n"+
						" got: %s\n"+
						"want: %s",
					got,
					want,
				)
			}
		})
		wg.Wait()
		tb.WithExpectEmpty().Expect(t.Context(), t,
			tlog.M.
				With(LKConnResponse, cr).
				Debug(LMConnResponseSent),
		)

	}
	sTry := func(t *testing.T, rErr error) {
		synctest.Test(t, func(t *testing.T) { try(t, rErr) })
	}

	/* Try an error and a not-error. */
	t.Run("error", func(t *testing.T) { sTry(t, syscall.EBADF) })
	t.Run("not_error", func(t *testing.T) { sTry(t, nil) })
}

// Do we handle not being able to talk back?
func TestSendConnResponse_Error(t *testing.T) {
	var (
		rErr   = syscall.ENOMEM
		c, s   = newTestJSONStreamPair(t)
		cr     = crsadapter.ConnResponse{Error: rErr.Error()}
		tb, sl = tlog.NewBuffer()
	)
	/* Won't actually get the response. */
	if err := c.CloseRead(); nil != err {
		t.Fatalf("Error closing client stearm for reading: %v", err)
	}
	/* Send the response. */
	if sendConnResponse(sl, s, rErr) {
		t.Errorf("sendConnResponse returned true on failure")
	}
	/* We'll kinda guess at this one, since it doesn't look exposed. */
	emsg := fmt.Sprintf("jsontext: write error: %v", io.ErrClosedPipe)
	/* Logs correct? */
	tb.WithExpectEmpty().Expect(t.Context(), t,
		tlog.M.
			With(LKConnResponse, cr).
			With(LKError, emsg).
			Warn(LMConnResponseError),
	)
}

// Do we timeout connections that don't send a request fast enough?
func TestServerHandle_ConnRequestTimeout(t *testing.T) {
	synctest.Test(t, testServerHandleConnRequestTimeout)
}
func testServerHandleConnRequestTimeout(t *testing.T) {
	m, tb, _, _, c, _ := newTestServer(t, nil)
	/* Read on the pipe, should time out eventually. */
	b, err := io.ReadAll(c)
	if 0 != len(b) {
		t.Errorf("Non-empty client read: %q", b)
	}
	if nil != err {
		t.Errorf("Client read error: %v", err)
	}
	/* Get a warning? */
	tb.WithExpectEmpty().Expect(t.Context(), t,
		m.
			With(LKError, ErrConnRequestTimeout).
			Warn(LMConnRequestError),
	)
}

// Do we timeout connections that don't send a request fast enough?
func TestServerHandle_IncorrectConnType(t *testing.T) {
	/* Try sends a request for a ConnType and expects wantErr. */
	try := func(t *testing.T, ct crsadapter.ConnType, wantErr error) {
		m, tb, _, _, c, _ := newTestServer(t, nil)

		/* Send a connection request. */
		js := crsadapter.NewStream(c)
		if err := js.Send(crsadapter.ConnRequest{
			ConnType: ct,
		}); nil != err {
			t.Fatalf("Error sending ConnRequest: %v", err)
		}

		/* Should get a response, a newline, and a disconnect. */
		var cr crsadapter.ConnResponse
		if err := js.DecodeNext(&cr); nil != err {
			t.Fatalf("Error getting ConnResponse")
		}
		if got, want := cr.Error, wantErr.Error(); got != want {
			t.Errorf(
				"Incorrect error message\n got: %s\nwant: %s",
				got,
				want,
			)
		}
		b, err := io.ReadAll(js)
		if got, want := string(b), ""; got != want {
			t.Errorf(
				"Incorrect leftover data received\n"+
					" got: %q\n"+
					"want: %q",
				got,
				want,
			)
		}
		if nil != err {
			t.Errorf("Expected error after ConnResponse: %v", err)
		}

		/* Get a warning? */
		rm := m.
			With(
				LKConnRequest,
				crsadapter.ConnRequest{ConnType: ct},
			)
		tb.WithExpectEmpty().Expect(t.Context(), t,
			rm.
				Debug(LMConnRequestReceived),
			m.
				With(LKError, wantErr).
				Warn(LMConnRequestError),
			m.
				With(
					LKConnResponse,
					crsadapter.ConnResponse{
						Error: wantErr.Error(),
					},
				).
				Debug(LMConnResponseSent),
		)
	}
	sTry := func(t *testing.T, ct crsadapter.ConnType, wantErr error) {
		synctest.Test(t, func(t *testing.T) {
			try(t, ct, wantErr)
		})
	}
	t.Run("empty", func(t *testing.T) {
		sTry(t, crsadapter.ConnTypeUnspecified, ErrUnspecifiedConnType)
	})
	t.Run("invalid", func(t *testing.T) {
		ct := crsadapter.ConnType(tlog.S("invalid"))
		sTry(t, ct, UnknownConnTypeError{ct})
	})
}

// Is a happy ConnResponse just {} ?
func TestSendConnResponse_HappyResponse(t *testing.T) {
	var (
		c, s   = newTestJSONStreamPair(t)
		tb, sl = tlog.NewBuffer()
		wg     sync.WaitGroup
	)
	/* Send the response. */
	wg.Go(func() {
		defer s.Close()
		if !sendConnResponse(sl, s, nil) {
			t.Errorf("sendConnResponse returned false")
		}
	})
	wg.Go(func() {
		b, err := io.ReadAll(c)
		if nil != err {
			t.Errorf("Error reading respones: %v", err)
		}
		if got, want := string(b), "{}\n"; got != want {
			t.Errorf(
				"ConnResponse JSON incorrect\n"+
					" got: %q\n"+
					"want :%q",
				got,
				want,
			)
		}
	})
	wg.Wait()

	/* Logs ok? */
	tb.WithExpectEmpty().Expect(t.Context(), t,
		tlog.M.
			With(LKConnResponse, struct{}{}).
			Debug(LMConnResponseSent),
	)
}
