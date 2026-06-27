package hsrv

/*
 * response_headers.go
 * Optional HTTP response headers, loaded from a file.
 * By J. Stuart McMurray
 * Created 20260627
 * Last Modified 20260627
 */

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// addResponseHeaders returns an http.Handler which adds response headers
// as found in the file fn, if not the empty string.
func addResponseHeaders(h http.Handler, fn string) (http.Handler, error) {
	/* Easy day if we've no file. */
	if "" == fn {
		return h, nil
	}
	/* Get the headers to add. */
	headers, err := loadResponseHeaders(fn)
	if nil != err {
		return nil, fmt.Errorf("reading headers from %s: %w", fn, err)
	}
	/* Wrap the handler. */
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		/* Add the headers. */
		for k, vs := range headers {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
		/* Pass to the real handler. */
		h.ServeHTTP(w, r)

	}), nil
}

// loadResponseHeaders returns an [http.Header] unmarshalled from the JSON in
// fn.
// If fn is the empty string, loadResponseHeaders returns nil, nil.
func loadResponseHeaders(fn string) (http.Header, error) {
	/* Easy day if we've no file. */
	if "" == fn {
		return nil, nil
	}

	/* Open the file with the JSON'd headers. */
	f, err := os.Open(fn)
	if nil != err {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	/* Unmarshal the JSON. */
	ret := make(http.Header)
	if err := json.NewDecoder(f).Decode(&ret); nil != err {
		return nil, fmt.Errorf("unJSONing headers: %w", err)
	}

	return ret, nil
}
