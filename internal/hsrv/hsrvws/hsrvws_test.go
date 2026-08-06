package hsrvws

/*
 * hsrvws_test.go
 * Tests for hsrvws.go
 * By J. Stuart McMurray
 * Created 20260807
 * Last Modified 20260807
 */

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/net/websocket"
)

// Can we work out if an HTTP request asks for a websocket?
func TestIsWebsocketUpgradeRequest_Yes(t *testing.T) {
	verdict := make(chan bool, 2)
	svr := httptest.NewServer(http.HandlerFunc(func(
		_ http.ResponseWriter,
		r *http.Request,
	) {
		defer close(verdict) /* Also for idempotency. */
		verdict <- IsWebsocketUpgradeRequest(r)
	}))
	defer svr.Close()

	/* Make a websocket connection. */
	u := "ws://" + strings.TrimPrefix(svr.URL, "http://")
	c, err := websocket.Dial(u, "", u)
	if nil == err {
		c.Close()
		t.Errorf("Websocket connected, but shouldn't have")
	} else if got, want := err.Error(),
		fmt.Sprintf("websocket.Dial %s: bad status", u); got != want {
		t.Errorf(
			"Unexpected error making websocket connection\n"+
				" got: %v\n"+
				"want: %v",
			got,
			want,
		)
	}
	/* The jury has returned. */
	if wasWS, ok := <-verdict; !ok {
		t.Errorf("Did not get a verdict")
	} else if !wasWS {
		t.Errorf("Websocket request not detected as websocket request")
	}
}

// Can we work out if an HTTP request doesn't ask for a websocket?
func TestIsWebsocketUpgradeRequest_No(t *testing.T) {
	verdict := make(chan bool, 2)
	svr := httptest.NewServer(http.HandlerFunc(func(
		_ http.ResponseWriter,
		r *http.Request,
	) {
		defer close(verdict) /* Also for idempotency. */
		verdict <- IsWebsocketUpgradeRequest(r)
	}))
	defer svr.Close()

	/* Make a non-websocket connection. */
	res, err := http.Get(svr.URL)
	if nil != err {
		t.Errorf("Error making GET request: %v", err)
	}
	defer res.Body.Close()

	/* The jury has returned. */
	if wasWS, ok := <-verdict; !ok {
		t.Errorf("Did not get a verdict")
	} else if wasWS {
		t.Errorf("HTTP request detected as websocket")
	}
}
