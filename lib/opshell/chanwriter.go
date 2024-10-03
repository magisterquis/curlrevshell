package opshell

/*
 * chanwriter.go
 * Turn a channel into an io.Writer
 * By J. Stuart McMurray
 * Created 20240423
 * Last Modified 20240423
 */

// ChanWriter turns a chan string into an io.Writer.
type ChanWriter chan<- string

// Write satisfies io.Writer.  It blocks until it can write and always returns
// nil, len(b).
func (cw ChanWriter) Write(b []byte) (int, error) {
	cw <- string(b)
	return len(b), nil
}
