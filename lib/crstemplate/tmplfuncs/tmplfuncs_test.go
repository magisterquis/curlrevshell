package tmplfuncs

/*
 * tmplfuncs_test.go
 * Tests for tmplfuncs.go
 * By J. Stuart McMurray
 * Created 20250205
 * Last Modified 20250830
 */

import (
	"net"
	"reflect"
	"testing"
)

func TestPort(t *testing.T) {
	for have, want := range map[string]string{
		"a.b.c:123": "123",
		"a.b.c":     "443",
	} {
		t.Run(have, func(t *testing.T) {
			if got, err := Port(have); nil != err {
				t.Fatalf("Error: %s", err)
			} else if got != want {
				t.Errorf(
					"Port incorrect:\n"+
						"have: %s\n"+
						" got: %s\n"+
						"want: %s",
					have,
					got,
					want,
				)
			}
		})
	}
}

func TestEnsurePort(t *testing.T) {
	cs := map[string]struct {
		addr string
		port string
		want string
	}{
		"bare_domain": {
			addr: "kittens.com",
			want: "kittens.com:443",
		},
		"default_port": {
			addr: "kittens.com",
			port: "4444",
			want: "kittens.com:4444",
		},
		"included_port/default_port": {
			addr: "kittens.com:5555",
			want: "kittens.com:5555",
		},
		"included_port/nondefault_port": {
			addr: "kittens.com:5555",
			port: "6666",
			want: "kittens.com:5555",
		},
	}
	for n, c := range cs {
		t.Run(n, func(t *testing.T) {
			if got := EnsurePort(c.port, c.addr); got != c.want {
				t.Errorf(
					"Incorrect address:\n"+
						"addr: %s\n"+
						"port: %s\n"+
						" got: %s\n"+
						"want: %s",
					c.addr,
					c.port,
					got,
					c.want,
				)
			}
		})
	}
}

func TestNoDefaultPort(t *testing.T) {
	for have, want := range map[string]string{
		"a.b.c:123":            "a.b.c:123",
		"a.b.c":                "a.b.c",
		"a.b.c:" + DefaultPort: "a.b.c",
		"@-=":                  "@-=",
	} {
		t.Run(have, func(t *testing.T) {
			if got := NoDefaultPort(have); got != want {
				t.Errorf(
					"Address incorrect:\n"+
						"have: %s\n"+
						" got: %s\n"+
						"want: %s",
					have,
					got,
					want,
				)
			}
		})
	}
}

func TestHasNoPort(t *testing.T) {
	for have, want := range map[string]bool{
		"a.b.c:123": false,
		"a.b.c":     true,
		"a:::b":     false,
	} {
		t.Run(have, func(t *testing.T) {
			_, _, err := net.SplitHostPort(have)
			if got := hasNoPort(err); got != want {
				t.Errorf(
					"hasNoPort incorrect:\n"+
						"have: %s\n"+
						" got: %t\n"+
						"want: %t\n"+
						" err: %s",
					have,
					got,
					want,
					err,
				)
			}
		})
	}
}

func TestMap(t *testing.T) {
	type ES struct {
		N int
		S string
	}
	have := []any{
		"foo", 1,
		"bar", true,
		"tridge", []byte{0x01, 0x02, 0x03},
		"baaz", "quux",
		"kittens", ES{N: 10, S: "moose"},
	}
	want := map[string]any{
		"foo":     1,
		"bar":     true,
		"tridge":  []byte{0x01, 0x02, 0x03},
		"baaz":    "quux",
		"kittens": ES{N: 10, S: "moose"},
	}
	if got, err := Map(have...); nil != err {
		t.Errorf("Error: %s", err)
	} else if !reflect.DeepEqual(got, want) {
		t.Errorf(
			"Incorrect map\nhave: %v\n got: %v\nwant: %v",
			have,
			got,
			want,
		)
	}
}

func TestMatchRE(t *testing.T) {
	for n, c := range map[string]struct {
		re      string
		s       string
		want    bool
		wantErr string
	}{"simple_match": {
		re:   "kittens",
		s:    "moose kittens zoomies!",
		want: true,
	}, "bad_regex": {
		re: "bad[regex",
		s:  "dummy",
		wantErr: "compiling regular expression \"bad[regex\": " +
			"error parsing regexp: missing closing ]: `[regex`",
	}, "no_match": {
		re: "kittens",
		s:  "moose k_i_t_t_e_n_s zoomies!",
	}} {
		t.Run(n, func(t *testing.T) {
			var gotErr string
			m, err := MatchRE(c.re, c.s)
			if nil != err {
				gotErr = err.Error()
			}
			if got, want := gotErr, c.wantErr; got != want {
				t.Errorf(
					"Incorrect error\n got: %s\nwant: %s",
					got,
					want,
				)
			}
			if got, want := m, c.want; got != want {
				t.Errorf(
					"Incorrect return\n got: %t\nwant: %t",
					got,
					want,
				)
			}
		})
	}
}
