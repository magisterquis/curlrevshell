package hsrv

/*
 * handlers.go
 * HTTP handlers
 * By J. Stuart McMurray
 * Created 20240324
 * Last Modified 20260804
 */

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/net/websocket"
)

const (
	// OneShellFlag is the flag we use to indicate we only want one shell.
	OneShellFlag = "one-shell"
)

// idParam is the named value in the path for the implant ID.
const idParam = "id"

type (
	// testNoLogHIKey disables logging HTTP info, if found in a context
	// during testing.
	testNoLogHIKey struct{}
	// testCloseFileBeforeStatKey causes fileHandler to close the file it
	// has open before calling Stat on it, for error injection.
	testCloseFileBeforeStatKey struct{}
	// testRemoteAddrKey points to a channel in a context on which the
	// websocket handler sends the requests's remote address.
	testWSRemoteAddrKey struct{}
)

// newMux returns a new ServeMux, ready to serve.
func (s *Server) newMux() *http.ServeMux {
	var (
		mux = http.NewServeMux()
		p   = s.params
	)

	/* Shell I/O handler. */
	mux.HandleFunc("/"+p.URLPaths.InOut, s.inOutHandler)
	mux.HandleFunc("/"+p.URLPaths.InOut+"/", s.inOutHandler)
	mux.HandleFunc("/"+p.URLPaths.InOut+"/{"+idParam+"}", s.inOutHandler)
	/* Shell input handler. */
	mux.HandleFunc("/"+p.URLPaths.In+"/{"+idParam+"}", s.inputHandler)
	/* Shell output handler. */
	mux.HandleFunc("/"+p.URLPaths.Out+"/{"+idParam+"}", s.outputHandler)
	/* Shell over websockets handler. */
	mux.HandleFunc("/"+p.URLPaths.Websocket, s.websocketHandler)
	mux.HandleFunc("/"+p.URLPaths.Websocket+"/", s.websocketHandler)
	mux.HandleFunc("/"+p.URLPaths.Websocket+"/{"+idParam+"}", s.websocketHandler)
	/* Callback script handler. */
	mux.HandleFunc("/"+p.URLPaths.Script, s.scriptHandler)
	mux.HandleFunc("/"+p.URLPaths.Script+"/", s.scriptHandler)

	/* If we're serving static files, do that. */
	if "" != p.StaticFilesDir {
		mux.HandleFunc("/", s.fileHandler)
	}

	return mux
}

// fileHandler logs and serves files.
func (s *Server) fileHandler(w http.ResponseWriter, r *http.Request) {
	sl := s.requestLogger(r).With(
		LKRequestedFile, r.URL.String(),
		LKStaticFilesDir, s.params.StaticFilesDir,
	)

	/* Work out what to send back. */
	s.rLogf(fileColor, r, "File requested: %s", r.URL)
	f, err := os.Open(s.params.StaticFilesDir)
	if nil != err {
		s.rErrorLogf(
			r,
			"Could not open %s: %s",
			s.params.StaticFilesDir,
			err,
		)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	/* Fault injection. */
	if testing.Testing() {
		if bv, ok := r.Context().Value(
			testCloseFileBeforeStatKey{},
		).(bool); ok && bv {
			f.Close()
		}
	}

	fi, err := f.Stat()
	if nil != err {
		s.rErrorLogf(
			r,
			"Could not get info about %s: %s",
			s.params.StaticFilesDir,
			err,
		)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	sl.Info(LMFileRequested)

	/* If we've just been given one file, send it for all requests. */
	if fi.Mode().IsRegular() {
		http.ServeContent(
			w,
			r,
			s.params.StaticFilesDir,
			fi.ModTime(),
			f,
		)
		return
	}

	/* For everything else, let the http library do the work. */
	http.FileServer(http.Dir(s.params.StaticFilesDir)).ServeHTTP(w, r)
}

// inputHandler sends input to a shell.
func (s *Server) inputHandler(w http.ResponseWriter, r *http.Request) {
	s.iob.HandleInput(
		r.Context(),
		s.requestLogger(r),
		r.PathValue(idParam),
		remoteHost(r),
		w,
	)
}

// outputHandler receives output from a shell.
func (s *Server) outputHandler(w http.ResponseWriter, r *http.Request) {
	s.iob.HandleOutput(
		r.Context(),
		s.requestLogger(r),
		r.PathValue(idParam),
		remoteHost(r),
		r.Body,
	)
}

// inOutHandler handles both input and output for a shell.
func (s *Server) inOutHandler(w http.ResponseWriter, r *http.Request) {
	if err := StartFullDuplex(w, r); nil != err {
		s.rErrorLogf(r, "Error starting duplex comms: %s", err)
		return
	}
	s.iob.HandleBidirectional(
		r.Context(),
		s.requestLogger(r),
		r.PathValue(idParam),
		remoteHost(r),
		newRequestRWC(w, r),
	)
}

// websocketHandler upgrades to a websocket and handles both input and output
// for a shell.
func (s *Server) websocketHandler(w http.ResponseWriter, r *http.Request) {
	/* Send a test r.RemoteAddr if asked. */
	if testing.Testing() {
		if ch, ok := r.Context().Value(
			testWSRemoteAddrKey{},
		).(chan string); ok {
			ch <- r.RemoteAddr
		}
	}
	websocket.Server{
		// Handshake makes sure that conf.Origin is set so we don't
		// end up with a nil pointer derefence if someone calls
		// RemoteAddr.
		Handshake: func(
			conf *websocket.Config,
			r *http.Request,
		) error {
			conf.Origin = &url.URL{Host: r.RemoteAddr}
			return nil
		},
		// Handler hooks up the websocket and the I/O Broker. */
		Handler: func(c *websocket.Conn) {
			s.iob.HandleBidirectional(
				r.Context(),
				s.requestLogger(r),
				r.PathValue(idParam),
				remoteHost(r),
				c,
			)
		},
	}.ServeHTTP(w, r)
}

// StartFullDuplex enables full duplex mode on w, if possible.  This is
// necessary for some clients which are waiting on a go-ahead.
func StartFullDuplex(w http.ResponseWriter, r *http.Request) error {
	rc := http.NewResponseController(w)

	/* Full duplex is required by real HTTP clients, but doesn't work
	with the handler-tester. */
	if err := rc.EnableFullDuplex(); nil != err {
		return fmt.Errorf("enabling full duplex: %w", err)
	}

	/* If we expect a 100 Continue, write it.  Ideally this would happen
	when we start reading, but there's no way to guarantee we'll read
	before we start writing, which will write a 200.

	Kinda fragile, see https://github.com/golang/go/issues/67555 */

	/* Write the header from the get-go.  Helps with clients waiting on
	a proper go-ahead. */
	if expects100Continue(r) {
		w.WriteHeader(http.StatusContinue)
	}

	if err := rc.Flush(); nil != err {
		return fmt.Errorf(
			"sending initial HTTP response header: %w",
			err,
		)
	}

	return nil
}

// requestLogger returns a log.Logger which has information about r.
func (s *Server) requestLogger(r *http.Request) *slog.Logger {
	sl := s.sl

	/* We may skip this for testing. */
	if testing.Testing() {
		if noHI, ok := r.Context().Value(
			testNoLogHIKey{},
		).(bool); ok && noHI {
			return sl
		}
	}

	/* Work out the SNI, which may or may not exyist. */
	var sni string
	if nil != r.TLS {
		sni = r.TLS.ServerName
	}
	/* Logger with ALL the info. */
	return sl.With(slog.Group(
		LKRequestInfo,
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

// expects100Continue attempts to determine if r expects a 100 Continue reply.
func expects100Continue(r *http.Request) bool {
	/* HTTP library may have done it for us, though it'd be nice to not
	rely on an unexported type. */
	if "*http.expectContinueReader" == reflect.TypeOf(r.Body).String() {
		return true
	}
	/* Look for it in headers.  Edge cases are abundant, though. */
	return strings.Contains(
		strings.ToLower(r.Header.Get("Expect")),
		"100-continue",
	)
}
