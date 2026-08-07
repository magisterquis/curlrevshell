package hsrv

/*
 * request_rwc.go
 * Turn an http.ResponseWriter and http.Request into an io.ReadWriteCloser
 * By J. Stuart McMurray
 * Created 20260729
 * Last Modified 20260807
 */

import (
	"errors"
	"io"
	"net/http"
	"os"
	"time"
)

// requestRWC wraps an [http.ResponseWriter] and [http.Request] into an
// [io.ReadWriteCloser] suitable for passing to
// [iobroker.Broker.HandleBidirectional].
type requestRWC struct {
	w      http.ResponseWriter
	r      io.ReadCloser /* http.Request.Body. */
	closer func() error
	flush  func() error
}

// newRequestRWC returns a new requestRWC wrapping w and r.
func newRequestRWC(w http.ResponseWriter, r *http.Request) requestRWC {
	rc := http.NewResponseController(w)
	return requestRWC{
		w: w,
		r: r.Body,
		closer: func() error {
			/* Unblocks the read on r.Body. */
			rc.SetReadDeadline(time.Now())
			err := r.Body.Close()
			if errors.Is(err, os.ErrDeadlineExceeded) {
				err = nil
			}
			return err
		},
		flush: rc.Flush,
	}
}

// Read reads from r.r.
func (r requestRWC) Read(p []byte) (n int, err error) {
	return r.r.Read(p)
}

// Write writes to r.w.
func (r requestRWC) Write(p []byte) (n int, err error) { return r.w.Write(p) }

// Close calls r.close, if not nil.
func (r requestRWC) Close() error {
	if nil != r.closer {
		return r.closer()
	}
	return nil
}

// Flush calls r.flush, if not nil.
func (r requestRWC) Flush() error {
	if nil != r.flush {
		return r.flush()
	}
	return nil
}

// CloseRead is a no-op.
func (r requestRWC) CloseRead() error { return nil }

// CloseWrite is a no-op.
func (r requestRWC) CloseWrite() error { return nil }
