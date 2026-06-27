package hsrv

/*
 * response_headers_test.go
 * Optional HTTP response headers, loaded from a file.
 * By J. Stuart McMurray
 * Created 20260627
 * Last Modified 20260627
 */

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/magisterquis/curlrevshell/internal/tlog"
)

// Can we add response headers?
func TestAddResponseHeaders(t *testing.T) {
	/* Random keys and values. */
	var (
		k1 = tlog.S("k1")
		k2 = tlog.S("k2")
		v1 = tlog.S("v1")
		v2 = tlog.S("v2")
		v3 = tlog.S("v3")
		v4 = tlog.S("v4")
	)
	// Try checks that addResponseHeaders' handler adds the response headrs
	// in the JSON string have.  If noFN is set, addResponseHeaders will
	// be passed an empty filename.
	try := func(
		t *testing.T,
		have string,
		noFN bool,
		want http.Header,
		wantErr error,
	) {
		/* File with our headers. */
		fn := filepath.Join(t.TempDir(), "h.json")
		if err := os.WriteFile(fn, []byte(have), 0660); nil != err {
			t.Fatalf("Error writing headers: %v", err)
		}
		if noFN {
			fn = ""
		}
		/* Handler to add headers. */
		h, err := addResponseHeaders(http.HandlerFunc(
			func(http.ResponseWriter, *http.Request) {},
		), fn)
		if nil != wantErr {
			if !errors.Is(err, wantErr) {
				t.Fatalf("Incorrect error: %v", err)
			}
			return
		}
		if nil != err {
			t.Fatalf("Error: %v", err)
		}
		/* Does it work? */
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
		if got := rr.Result().Header; !reflect.DeepEqual(got, want) {
			t.Errorf(
				"Returned headers incorrect\n"+
					"have: %s\n"+
					" got: %#v\n"+
					"want: %#v",
				have,
				got,
				want,
			)
		}
	}
	for n, c := range map[string]struct {
		have    string /* JSON */
		noFN    bool   /* Empty filename? */
		want    http.Header
		wantErr error
	}{"empty_filename": {
		noFN: true,
		want: http.Header{},
	}, "empty_file": {
		wantErr: io.EOF,
	}, "empty_object": {
		have: `{}`,
		want: http.Header{},
	}, "single_value": {
		have: toJSON(t, http.Header{
			k1: []string{v1},
			k2: []string{v2},
		}),
		want: http.Header{
			http.CanonicalHeaderKey(k1): []string{v1},
			http.CanonicalHeaderKey(k2): []string{v2},
		},
	}, "multiple_values": {
		have: toJSON(t, http.Header{
			k1: []string{v1, v2},
			k2: []string{v3, v4},
		}),
		want: http.Header{
			http.CanonicalHeaderKey(k1): []string{v1, v2},
			http.CanonicalHeaderKey(k2): []string{v3, v4},
		},
	}} {
		t.Run(n, func(t *testing.T) {
			try(t, c.have, c.noFN, c.want, c.wantErr)
		})
	}
}

// Are we ok if the filename is empty or we can't open the file?
func TestLoadResponseHeaders_NoResponseHeaders(t *testing.T) {
	t.Run("does not exist", func(t *testing.T) {
		var (
			fn   = filepath.Join(t.TempDir(), "does_not_exist")
			want = os.ErrNotExist
		)
		if _, got := loadResponseHeaders(fn); !errors.Is(got, want) {
			t.Errorf(
				"Incorrect error opening a missing file\n"+
					" got: %s\n"+
					"want: %s",
				got,
				want,
			)
		}
	})

	t.Run("empty filename", func(t *testing.T) {
		got, err := loadResponseHeaders("")
		if nil != err {
			t.Errorf("Unexpected error: %v", err)
		}
		if nil != got {
			t.Errorf("Unexpected headers: %#v", got)
		}
	})

}

// Shouldn't have ResponseHeadersEnvVar set during tests.
func TestResponseHeadersEnvVar_NotSet(t *testing.T) {
	if got := os.Getenv(ResponseHeadersEnvVar); "" != got {
		t.Fatalf(
			"%s set during testing: %s",
			ResponseHeadersEnvVar,
			got,
		)
	}
}

// toJSON returns v as JSON.
func toJSON(t *testing.T, v any) string {
	b, err := json.Marshal(v)
	if nil != err {
		t.Fatalf("Error marshalling %#v: %v", v, err)
	}
	return string(b)
}
