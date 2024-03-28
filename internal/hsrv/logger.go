package hsrv

/*
 * logger.go
 * io.Writer which sends pink messages to opshell.Shell
 * By J. Stuart McMurray
 * Created 20240324
 * Last Modified 20240328
 */

import (
	"fmt"
	"net"
	"net/http"

	"github.com/magisterquis/curlrevshell/lib/opshell"
)

// Colors for log things
const (
	ErrorColor     = opshell.ColorRed
	FileColor      = opshell.ColorBlue
	ScriptColor    = opshell.ColorCyan
	ConnectedColor = opshell.ColorGreen
)

// Printf sends a colored message to the shell.  The shell will ensure it ends
// in a newline.  No timestamp will be printed before the line.
func (s *Server) Printf(color opshell.Color, format string, v ...any) {
	/* Send the message to be logged. */
	s.och <- opshell.CLine{
		Color:       color,
		Line:        fmt.Sprintf(format, v...),
		NoTimestamp: true,
	}
}

// Logf sends a colered message to the shell.
func (s *Server) Logf(color opshell.Color, format string, v ...any) {
	/* Send the message to be logged. */
	s.och <- opshell.CLine{
		Color: color,
		Line:  fmt.Sprintf(format, v...),
	}
}

// RLogf sends a colored message to the shell with the requetsor's IP address.
func (s *Server) RLogf(color opshell.Color, r *http.Request, format string, v ...any) {
	s.Logf(color, fmt.Sprintf(
		"[%s] %s",
		remoteHost(r),
		fmt.Sprintf(format, v...),
	))
}

// ErrorLogf sends a error message back.
func (s *Server) ErrorLogf(format string, v ...any) {
	s.Logf(ErrorColor, format, v...)
}

// RErrorLogf sends a pink message to the shell with r's remote address.
func (s *Server) RErrorLogf(r *http.Request, format string, v ...any) {
	s.ErrorLogf(fmt.Sprintf(
		"[%s] %s",
		remoteHost(r),
		fmt.Sprintf(format, v...),
	))
}

// remoteHost attempts to get just the host part of the remote address.  If
// it fails, it returns the whole thing.
func remoteHost(r *http.Request) string {
	if h, _, err := net.SplitHostPort(r.RemoteAddr); nil == err {
		return h
	}
	return r.RemoteAddr
}

// pinkSender is an io.Writer which sends pink opshell.CLines.
type pinkSender struct{ och chan<- opshell.CLine }

// Write implements io.Writer.  It always returns len(p), nil.
func (ps pinkSender) Write(p []byte) (n int, err error) {
	ps.och <- opshell.CLine{Color: ErrorColor, Line: string(p)}
	return len(p), nil
}
