package tmplfuncs

/*
 * tmplfuncs_test.go
 * Tests for tmplfuncs.go
 * By J. Stuart McMurray
 * Created 20250205
 * Last Modified 20251008
 */

import (
	"bytes"
	"go/types"
	"net"
	"reflect"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"
	"text/template"

	"golang.org/x/tools/go/packages"
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

// Are all of the exported functions in TemplateFuncs?
func Test_AllFuncsAvailable(t *testing.T) {
	/* Regex to get the last dot-separated bit. */
	re := regexp.MustCompile(`.*\.`)
	t.Run("regex", func(t *testing.T) {
		have := "foo.bar/tridge/baaz.quux"
		if got, want := re.ReplaceAllString(
			have,
			"",
		), "quux"; got != want {
			t.Fatalf(
				"Regex failed\n"+
					"regex: %s\n"+
					" have: %s\n"+
					"  got: %s\n"+
					" want: %s",
				re,
				have,
				got,
				want,
			)
		}
	})

	/* Get the functions in TemplateFuncs. */
	var mapFuncs []string
	for k, v := range reflect.ValueOf(TemplateFuncs).Seq2() {
		fn := re.ReplaceAllString(runtime.FuncForPC(
			uintptr(v.Elem().UnsafePointer()),
		).Name(), "")
		/* Is the key the same as the function name? */
		if got, want := k.String(), strings.ToLower(fn); got != want {
			t.Errorf(
				"Map key incorrect\n"+
					" got: %s\n"+
					"want: %s",
				got,
				want,
			)
		}
		mapFuncs = append(mapFuncs, fn)
	}
	slices.Sort(mapFuncs)

	/* Get hold of the current package. */
	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedTypes,
	}, ".")
	if nil != err {
		t.Fatalf("Error loading current package: %s", err)
	} else if got, want := len(pkgs), 1; got != want {
		t.Fatalf(
			"Found %d packages in current directory, expected %d",
			got,
			want,
		)
	} else if nil == pkgs[0].Types {
		t.Fatalf("Package type information isn't available")
	}

	/* Work out the package's exported functions. */
	var exportedFuncs []string
	scope := pkgs[0].Types.Scope()
	if nil == scope {
		t.Fatalf("Failed to get package's scope")
	}
	for _, n := range scope.Names() {
		/* Only care about exported functions. */
		obj := scope.Lookup(n)
		if nil == obj {
			t.Errorf("Failed to look up info about %q", n)
			continue
		}
		if _, ok := obj.(*types.Func); !ok || !obj.Exported() {
			continue
		}
		exportedFuncs = append(exportedFuncs, n)
	}
	if 0 == len(exportedFuncs) {
		t.Fatalf("Did not find any exported functions")
	}
	slices.Sort(exportedFuncs)

	/* Did we get the right functions? */
	if got, want := mapFuncs, exportedFuncs; !slices.Equal(got, want) {
		t.Errorf(
			"TemplateFuncs contains incorrect functions\n"+
				" got: %q\n"+
				"want: %q",
			got,
			want,
		)
	}
}

// Can we call the functions in a template.
func Test_FunctionCall(t *testing.T) {
	for n, c := range map[string]struct {
		template string
		dot      string
		want     string
	}{"sanity_check": {
		template: "kittens",
		want:     "kittens",
	}, "ensureport/default_port": {
		template: `{{ . | ensureport "" }}`,
		dot:      "kittens.com",
		want:     "kittens.com:443",
	}, "ensureport/set_port": {
		template: `{{ . | ensureport "123" }}`,
		dot:      "kittens.com",
		want:     "kittens.com:123",
	}, "ensureport/port_exists": {
		template: `{{ . | ensureport "123" }}`,
		dot:      "kittens.com:234",
		want:     "kittens.com:234",
	}, "ensureport/no_address": {
		template: `{{ . | ensureport "123" }}`,
	}, "host/empty": {
		template: `{{ . | host }}`,
	}, "host/domain_and_port": {
		template: `{{ . | host }}`,
		dot:      "kittens.com:123",
		want:     "kittens.com",
	}, "host/only_domain": {
		template: `{{ . | host }}`,
		dot:      "kittens.com",
		want:     "kittens.com",
	}, "host/ip_and_port": {
		template: `{{ . | host }}`,
		dot:      "1.2.3.4:123",
		want:     "1.2.3.4",
	}, "host/only_ip": {
		template: `{{ . | host }}`,
		dot:      "1.2.3.4",
		want:     "1.2.3.4",
	}, "map": {
		template: `{{ with map "dot" . "ks" "v" "kn" 2 "kb" true}}
		{{- .dot}} {{ .ks }} {{ .kn }} {{ .kb }}{{ end }}`,
		dot:  "d0t",
		want: "d0t v 2 true",
	}, "matchre/match": {
		template: `{{ . | matchre "\\d+$" }}`,
		dot:      "abc123",
		want:     "true",
	}, "matchre/no_match": {
		template: `{{ . | matchre "\\d+$" }}`,
		dot:      "kittens",
		want:     "false",
	}, "nodefaultport/remove_port": {
		template: `{{ . | nodefaultport }}`,
		dot:      "kittens.com:443",
		want:     "kittens.com",
	}, "nodefaultport/no_port": {
		template: `{{ . | nodefaultport }}`,
		dot:      "kittens.com",
		want:     "kittens.com",
	}, "nodefaultport/not_default_port": {
		template: `{{ . | nodefaultport }}`,
		dot:      "kittens.com:123",
		want:     "kittens.com:123",
	}, "port/empty": {
		template: `{{ . | port }}`,
		want:     "443",
	}, "port/no_port": {
		template: `{{ . | port }}`,
		dot:      "kittens.com",
		want:     "443",
	}, "port/got_port": {
		template: `{{ . | port }}`,
		dot:      "kittens.com:123",
		want:     "123",
	}} {
		t.Run(n, func(t *testing.T) {
			tmpl, err := template.New("").
				Funcs(TemplateFuncs).
				Parse(c.template)
			if nil != err {
				t.Fatalf("Error parsing template: %s", err)
			}
			b := new(bytes.Buffer)
			if err := tmpl.Execute(b, c.dot); nil != err {
				t.Fatalf("Error executing template: %s", err)
			}
			if got, want := b.String(), c.want; got != want {
				t.Fatalf(
					"Incorrect template output\n"+
						"template: %s\n"+
						"     dot: %s\n"+
						"     got: %s\n"+
						"    want: %s",
					c.template,
					c.dot,
					got,
					c.want,
				)
			}
		})
	}
}
