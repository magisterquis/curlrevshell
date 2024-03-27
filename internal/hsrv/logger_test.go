package hsrv

/*
 * logger_test.go
 * Tests for logger.go
 * By J. Stuart McMurray
 * Created 20240324
 * Last Modified 20240325
 */

import (
	"curlrevshell/lib/opshell"
	"net/http/httptest"
	"testing"
)

func TestServerLogf(t *testing.T) {
	_, och, s := newTestServer(t)
	haveColor := opshell.ColorBlue
	s.Logf(haveColor, "Hello, %s!", "Kittens")
	want := opshell.CLine{
		Color: haveColor,
		Line:  "Hello, Kittens!",
	}
	if got := <-och; got != want {
		t.Errorf(
			"Incorrect message:\n got: %#v\nwant: %#v",
			got,
			want,
		)
	}
}

func TestServerRLogf(t *testing.T) {
	_, och, s := newTestServer(t)
	haveColor := opshell.ColorBlue
	haveR := httptest.NewRequest("GET", "http://127.0.0.1:4444", nil)
	s.RLogf(haveColor, haveR, "Hello, %s!", "Kittens")
	want := opshell.CLine{
		Color: haveColor,
		Line:  "[192.0.2.1] Hello, Kittens!",
	}
	if got := <-och; got != want {
		t.Errorf(
			"Incorrect message:\n got: %#v\nwant: %#v",
			got,
			want,
		)
	}
}

func TestServerErrorLogf(t *testing.T) {
	_, och, s := newTestServer(t)
	s.ErrorLogf("Hello, %s!", "Kittens")
	want := opshell.CLine{
		Color: ErrorColor,
		Line:  "Hello, Kittens!",
	}
	if got := <-och; got != want {
		t.Errorf(
			"Incorrect message:\n got: %#v\nwant: %#v",
			got,
			want,
		)
	}
}

func TestServerRErrorLogf(t *testing.T) {
	_, och, s := newTestServer(t)
	haveR := httptest.NewRequest("GET", "http://127.0.0.1:4444", nil)
	s.RErrorLogf(haveR, "Hello, %s!", "Kittens")
	want := opshell.CLine{
		Color: ErrorColor,
		Line:  "[192.0.2.1] Hello, Kittens!",
	}
	if got := <-och; got != want {
		t.Errorf(
			"Incorrect message:\n got: %#v\nwant: %#v",
			got,
			want,
		)
	}
}
