package hsrv

/*
 * errors.go
 * Error types and values
 * By J. Stuart McMurray
 * Created 20260801
 * Last Modified 20260801
 */

import (
	"errors"
	"fmt"
	"testing"

	"github.com/magisterquis/curlrevshell/internal/tlog"
)

// Does a listenError's error message look right?
func TestListenErrorError(t *testing.T) {
	var (
		addr = tlog.S("addr")
		err  = errors.New(tlog.S("err"))

		have = listenError{
			Addr: addr,
			Err:  err,
		}
		want = fmt.Sprintf("listening on %s: %s", addr, err)
	)

	if got := have.Error(); got != want {
		t.Errorf(
			"Incorrect string form of error\n"+
				"have: %v\n"+
				" got: %s\n"+
				"want: %s",
			have,
			got,
			want,
		)
	}
}
