package opshell

/*
 * stdiorw.go
 * Wrap stdio in an io.ReadWriter
 * By J. Stuart McMurray
 * Created 20240323
 * Last Modified 20240324
 */

import "os"

// stdioRW combines os.Stdin and os.Stdout into an io.ReadWriter.
type stdioRW struct {
}

func (stdioRW) Read(p []byte) (int, error)  { return os.Stdin.Read(p) }
func (stdioRW) Write(p []byte) (int, error) { return os.Stdout.Write(p) }
