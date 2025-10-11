package crstemplate

/*
 * request_test.go
 * Tests for request.go
 * By J. Stuart McMurray
 * Created 20250126
 * Last Modified 20250613
 */

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/magisterquis/curlrevshell/lib/crstemplate/tmplfuncs"
)

var (
	testUsername = "test_username"
	testPassword = "test_password"
	testID       = "testID"
)

// newTestParamsWithoutRequest returns a new Params to which an HTTP request
// hasn't been added.
func newTestParamsWithoutRequest(t *testing.T) Params {
	return Params{
		ListenAddress:     "1.2.3.4:5",
		PubkeyFP:          "testFPtestFPtestFPtestFPtestFPtestFPtestFPx=",
		CallbackAddresses: []string{"example.com:6", "7.8.9.0:1"},
		URLPaths: URLPaths{
			In:     "testI",
			InOut:  "testIO",
			Out:    "testO",
			Script: "testC",
		},
		StaticFilesDir: t.TempDir(),
	}
}

// newTestRequest returns an in-flight HTTP request, which will be active
// until t's cleanup methods are called.
// The request will use testUsername and testPassword for basic auth.
func newTestRequest(t *testing.T) *http.Request {
	/* Server which returns its request. */
	var (
		ctx, cancel = context.WithCancel(context.Background())
		rch         = make(chan *http.Request, 1)
		wg          sync.WaitGroup
		svr         = httptest.NewUnstartedServer(http.HandlerFunc(func(
			_ http.ResponseWriter,
			r *http.Request,
		) {
			rch <- r
			<-ctx.Done()
		}))
		res  *http.Response
		rerr error
	)
	/* Shutdown the request and server when the test is done. */
	t.Cleanup(func() {
		cancel()
		svr.Close()
		wg.Wait()
		if nil == rerr {
			if http.StatusOK != res.StatusCode {
				t.Fatalf(
					"Non-OK status generating request: %s",
					res.Status,
				)
			}
			res.Body.Close()
		} else {
			t.Fatalf("Error generating request: %s", rerr)
		}
	})

	/* Grab the request. */
	svr.StartTLS()
	wg.Add(1)
	go func() {
		defer wg.Done()
		res, rerr = svr.Client().Get(fmt.Sprintf(
			"https://%s:%s@%s",
			testUsername,
			testPassword,
			svr.Listener.Addr().String(),
		))
	}()

	return <-rch
}

func newTestParams(t *testing.T) Params {
	p := newTestParamsWithoutRequest(t)
	p.ID = testID
	var err error
	if p, err = AddRequest(p, newTestRequest(t)); nil != err {
		t.Fatalf("Error adding request to params: %s", err)
	}
	return p
}

func TestNewTestRequest(t *testing.T) {
	t.Run("smoketest", func(t *testing.T) { newTestRequest(t) })
	t.Run("creds", func(t *testing.T) {
		r := newTestRequest(t)
		u, p, ok := r.BasicAuth()
		if !ok {
			t.Fatalf("Missing creds")
		}
		if testUsername != u {
			t.Fatalf(
				"Incorrect username\n got: %s\nwant: %s",
				u,
				testUsername,
			)
		}
		if testPassword != p {
			t.Fatalf(
				"Incorrect password\n got: %s\nwant: %s",
				p,
				testPassword,
			)
		}
	})

}

func TestAddRequest(t *testing.T) {
	var (
		ran       bool
		wantAddr  string
		wantUser  = "kittens"
		wantPass  = "moose"
		wantPath  = "/zoomies"
		addReqErr error
	)
	svr := httptest.NewTLSServer(http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		ran = true
		got, err := AddRequest(newTestParamsWithoutRequest(t), r)
		if nil != err {
			addReqErr = err
			w.WriteHeader(http.StatusInternalServerError)
		}

		/* Make sure we get an ID. */
		if "" == got.ID {
			t.Errorf("No ID set")
		}
		/* Make sure we got the right C2 Address. */
		if got, want := got.C2Addr, wantAddr; got != want {
			t.Errorf(
				"Incorrect C2 Address\n got: %s\nwant: %s",
				got,
				want,
			)
		}
		/* Make sure we can get the request path. */
		if got, want := got.Request.URL.Path, wantPath; got != want {
			t.Errorf(
				"Incorrect request path:\n got: %s\nwant: %s",
				got,
				want,
			)
		}
		/* Make sure basic auth works. */
		if !got.BasicAuth.Ok {
			t.Errorf("Creds not found")
		} else if want, gotS :=
			wantUser, got.BasicAuth.Username; gotS != want {
			t.Errorf(
				"Incorrect username:\n got: %s\nwant: %s",
				gotS,
				want,
			)
		} else if want, gotS :=
			wantPass, got.BasicAuth.Password; gotS != want {
			t.Errorf(
				"Incorrect password:\n got: %s\nwant: %s",
				gotS,
				want,
			)
		}
	}))
	t.Cleanup(svr.Close)
	/* Work out the server's address. */
	u, err := url.Parse(svr.URL)
	if nil != err {
		t.Fatalf("Error parsing server's URL %s: %s", svr.URL, err)
	}
	wantAddr = net.JoinHostPort(u.Hostname(), u.Port())
	/* Send a request and make sure it ran. */
	res, err := svr.Client().Get(fmt.Sprintf(
		"https://%s:%s@%s%s",
		wantUser,
		wantPass,
		wantAddr,
		wantPath,
	))
	if nil != err {
		t.Fatalf("Error making request: %s", err)
	}
	res.Body.Close()
	/* Make sure things worked nicely. */
	if !ran {
		t.Fatalf("Handler did not run")
	}
	if nil != addReqErr {
		t.Errorf("AddRequest error: %s", err)
	} else if http.StatusOK != res.StatusCode {
		t.Errorf("Unexpected non-OK status: %s", res.Status)
	}
}

// Make sure we can't double-add requests to Params.
func TestAddRequest_ExistingRequest(t *testing.T) {
	if _, err := AddRequest(
		newTestParams(t),
		newTestRequest(t),
	); nil == err {
		t.Fatalf("Second request add returned nil error")
	} else if want := ErrRequestAlreadyAdded; !errors.Is(err, want) {
		t.Fatalf(
			"Incorrect error:\n got: %s\nwant: %s",
			err,
			want,
		)
	}
}

func TestC2Addr(t *testing.T) {
	cs := map[string]struct {
		req         func(s *httptest.Server) (*http.Request, error)
		want        string /* empty for server's ip:port. */
		wantAddPort bool   /* Add server's port to want. */
	}{
		"simple_URL/no_port": {
			req: func(s *httptest.Server) (*http.Request, error) {
				req, err := http.NewRequest(
					http.MethodGet,
					s.URL,
					nil,
				)
				if nil != err {
					return nil, err
				}
				req.Host = "moose.com"
				return req, nil
			},
			want: "moose.com",
		},
		"simple_URL/with_port": {
			req: func(s *httptest.Server) (*http.Request, error) {
				return http.NewRequest(
					http.MethodGet,
					s.URL,
					nil,
				)
			},
		},
		"query_param/no_port": {
			req: func(s *httptest.Server) (*http.Request, error) {
				return http.NewRequest(
					http.MethodGet,
					s.URL+"?"+C2Param+"=moose.com",
					nil,
				)
			},
			want: "moose.com",
		},
		"query_param/with_port": {
			req: func(s *httptest.Server) (*http.Request, error) {
				return http.NewRequest(
					http.MethodGet,
					s.URL+"?"+C2Param+"=moose.com:12",
					nil,
				)
			},
			want: "moose.com:12",
		},
		"sni": {
			req: func(s *httptest.Server) (*http.Request, error) {
				req, err := http.NewRequest(
					http.MethodGet,
					"https://zoomies.example.com",
					nil,
				)
				if nil != err {
					return nil, err
				}
				req.Host = testingIgnoreHostHeader
				return req, nil
			},
			want:        "zoomies.example.com",
			wantAddPort: true,
		},
		"header/no_port": {
			req: func(s *httptest.Server) (*http.Request, error) {
				req, err := http.NewRequest(
					http.MethodGet,
					s.URL,
					nil,
				)
				if nil != err {
					return nil, err
				}
				req.Header.Set(C2Param, "moose.com")
				return req, nil
			},
			want: "moose.com",
		},
		"header/with_port": {
			req: func(s *httptest.Server) (*http.Request, error) {
				req, err := http.NewRequest(
					http.MethodGet,
					s.URL,
					nil,
				)
				if nil != err {
					return nil, err
				}
				req.Header.Set(C2Param, "moose.com:23")
				return req, nil
			},
			want: "moose.com:23",
		},
		"body/no_port": {
			req: func(s *httptest.Server) (*http.Request, error) {
				req, err := http.NewRequest(
					http.MethodPost,
					s.URL,
					strings.NewReader(url.Values{
						C2Param: []string{
							"kittens.com",
						},
					}.Encode()),
				)
				if nil != err {
					return nil, err
				}
				req.Header.Set(
					"Content-Type",
					"application/x-www-form-urlencoded",
				)
				return req, nil
			},
			want: "kittens.com",
		},
		"body/with_port": {
			req: func(s *httptest.Server) (*http.Request, error) {
				req, err := http.NewRequest(
					http.MethodPost,
					s.URL,
					strings.NewReader(url.Values{
						C2Param: []string{
							"kittens.com:56",
						},
					}.Encode()),
				)
				if nil != err {
					return nil, err
				}
				req.Header.Set(
					"Content-Type",
					"application/x-www-form-urlencoded",
				)
				return req, nil
			},
			want: "kittens.com:56",
		},
	}
	for n, c := range cs {
		t.Run(n, func(t *testing.T) {
			/* Server which saves the request's C2 address. */
			var (
				got  string
				herr error
			)
			svr := httptest.NewTLSServer(http.HandlerFunc(func(
				_ http.ResponseWriter,
				r *http.Request,
			) {
				got, herr = c2Addr(r, localAddrFromRequest(r))
			}))
			t.Cleanup(svr.Close)
			/* Work out what we'd like. */
			u, err := url.Parse(svr.URL)
			svrAddr := net.JoinHostPort(
				u.Hostname(),
				u.Port(),
			)

			if nil != err {
				t.Fatalf(
					"Error parsing server URL %s: %s",
					svr.URL,
					err,
				)
			}
			if "" == c.want {
				c.want = svrAddr
			} else if c.wantAddPort {
				c.want = tmplfuncs.EnsurePort(u.Port(), c.want)
			}
			/* Make the request. */
			req, err := c.req(svr)
			if nil != err {
				t.Fatalf("Error generating request: %s", err)
			}
			hc := svr.Client()
			hc.Transport.(*http.Transport).DialContext = func(
				ctx context.Context,
				network string,
				_ string,
			) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(
					ctx,
					network,
					svrAddr,
				)
			}
			res, err := hc.Do(req)
			if nil != err {
				t.Fatalf("Error making request: %s", err)
			}
			defer res.Body.Close()
			/* Make sure it worked. */
			if http.StatusOK != res.StatusCode {
				t.Errorf("Non-OK status: %s", res.Status)
			}
			/* See if we got the right address. */
			if nil != herr {
				t.Fatalf("Error getting C2 address: %s", herr)
			} else if got != c.want {
				t.Fatalf(
					"Incorrect address:\n"+
						"server: %s\n"+
						"   got: %s\n"+
						"  want: %s\n",
					svr.URL,
					got,
					c.want,
				)
			}
		})
	}
}
