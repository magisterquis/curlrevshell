package hsrv

/*
 * handlers.go
 * HTTP handlers
 * By J. Stuart McMurray
 * Created 20240324
 * Last Modified 20241013
 */

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"testing"
)

// Log messages and keys.
const (
	LMFileRequested = "File requested"

	LKStaticFilesDir = "static_files_dir"
)

const (
	// ClosingListenerMessage is what we print to tell the user we're
	// closing the listener after getting a shell when we've also got
	// -one-shell.
	ClosingListenerMessage = "Closing listener, because -" + OneShellFlag

	// OneShellFlag is the flag we use to indicate we only want one shell.
	OneShellFlag = "one-shell"
)

const (
	// idParam is the named value in the path for the implant ID.
	idParam = "id"
)

// newMux returns a new ServeMux, ready to serve.
func (s *Server) newMux() *http.ServeMux {
	mux := http.NewServeMux()

	/* Shellish handlers. */
	mux.HandleFunc("/i/{"+idParam+"}", s.inputHandler)  /* Shell input. */
	mux.HandleFunc("/o/{"+idParam+"}", s.outputHandler) /* Shell output. */
	mux.HandleFunc("/io", s.inOutHandler)               /* Shell I/O. */
	mux.HandleFunc("/io/", s.inOutHandler)              /* Shell I//O. */
	mux.HandleFunc("/c", s.scriptHandler)               /* Callback script. */

	/* If we're serving static files, do that. */
	if "" != s.fdir {
		mux.HandleFunc("/", s.fileHandler)
	}

	return mux
}

// fileHandler logs and serves files.
func (s *Server) fileHandler(w http.ResponseWriter, r *http.Request) {
	sl := s.requestLogger(r).With(LKStaticFilesDir, s.fdir)

	/* Work out what to send back. */
	s.RLogf(FileColor, r, "File requested: %s", r.URL)
	f, err := os.Open(s.fdir)
	if nil != err {
		s.RErrorLogf(r, "Could not open %s: %s", s.fdir, err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if nil != err {
		s.RErrorLogf(r, "Could not get info about %s: %s", s.fdir, err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	sl.Info(LMFileRequested)

	/* If we've just been given one file, send it for all requests. */
	if fi.Mode().IsRegular() {
		http.ServeContent(w, r, s.fdir, fi.ModTime(), f)
		return
	}

	/* For everything else, let the http library do the work. */
	http.FileServer(http.Dir(s.fdir)).ServeHTTP(w, r)
}

// inputHandler sends input to a shell.
func (s *Server) inputHandler(w http.ResponseWriter, r *http.Request) {
	s.iob.ConnectIn(
		r.Context(),
		s.requestLogger(r),
		remoteHost(r),
		w,
		r.PathValue(idParam),
	)
}

// outputHandler receives output from a shell.
func (s *Server) outputHandler(w http.ResponseWriter, r *http.Request) {
	s.iob.ConnectOut(
		r.Context(),
		s.requestLogger(r),
		remoteHost(r),
		r.Body,
		r.PathValue(idParam),
	)
}

// inOutHandler handles both input and output for a shell.
func (s *Server) inOutHandler(w http.ResponseWriter, r *http.Request) {
	rc := http.NewResponseController(w)
	/* Full duplex is required by real HTTP clients, but doesn't work
	with the handler-tester. */
	if err := rc.EnableFullDuplex(); nil != err &&
		!(testing.Testing() && errors.Is(err, http.ErrNotSupported)) {
		s.RErrorLogf(r, "Error enabling duplex comms: %s", err)
		return
	}
	/* Write the header from the get-go.  Helps with clients waiting on
	a proper go-ahead. */
	if err := rc.Flush(); nil != err {
		s.RErrorLogf(
			r,
			"Error sending initial HTTP response header: %s",
			err,
		)
	}
	s.iob.ConnectInOut(
		r.Context(),
		s.requestLogger(r),
		remoteHost(r),
		w,
		r.Body,
	)
}

// requestLogger returns a log.Logger which has information about r.
func (s *Server) requestLogger(r *http.Request) *slog.Logger {
	/* Work out the SNI, which may or may not exyist. */
	var sni string
	if nil != r.TLS {
		sni = r.TLS.ServerName
	}
	/* Logger with ALL the info. */
	return s.sl.With(slog.Group(
		"http_request",
		"remote_addr", r.RemoteAddr,
		"method", r.Method,
		"request_uri", r.RequestURI,
		"protocol", r.Proto,
		"host", r.Host,
		"sni", sni,
		"user_agent", r.UserAgent(),
		"id", r.PathValue(idParam),
	))
}
