package hsrv

/*
 * logger_test.go
 * Tests for logger.go
 * By J. Stuart McMurray
 * Created 20240324
 * Last Modified 20270801
 */

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/magisterquis/curlrevshell/internal/tlog"
	"github.com/magisterquis/curlrevshell/lib/opshell"
)

// Can we return the hostname from an address without a port?
func TestRemoteHost_NoPort(t *testing.T) {
	var (
		req  = httptest.NewRequest(http.MethodGet, "/", nil)
		host = tlog.S("host")
	)

	/* Request needs no port.  Not likely in practice, but for just in
	case. */

	/* Can we split it properly? */
	req.RemoteAddr = host
	if got, want := remoteHost(req), host; got != want {
		t.Errorf(
			"Incorrect host\nhave: %s\n got: %s\nwant: %s",
			req.RemoteAddr,
			got,
			want,
		)
	}
}

// Test ALL the logging functions. */
func TestServer_Logging(t *testing.T) {
	_, _, och, _, _, s := newTestServer(t.Context(), t, nil)
	var (
		elf  = tlog.S("elf")
		lf   = tlog.S("lf")
		ra   = tlog.S("host")
		relf = tlog.S("relf")
		rlf  = tlog.S("rlf")

		req = &http.Request{RemoteAddr: ra}
		tag = "[" + ra + "] "
	)
	s.errorLogf("errorLogf: %s", elf)
	s.logf(opshell.ColorYellow, "logf: %s", lf)
	s.rErrorLogf(req, "rErrorLogf: %s", relf)
	s.rLogf(opshell.ColorCyan, req, "rLogf: %s", rlf)
	opshell.ExpectShellMessages(t, och, []opshell.CLine{{
		Color: opshell.ColorRed,
		Line:  "errorLogf: " + elf,
	}, {
		Color: opshell.ColorYellow,
		Line:  "logf: " + lf,
	}, {
		Color: opshell.ColorRed,
		Line:  tag + "rErrorLogf: " + relf,
	}, {
		Color: opshell.ColorCyan,
		Line:  tag + "rLogf: " + rlf,
	}}...)
}
