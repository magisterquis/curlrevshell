package crstemplate

/*
 * params.go
 * Tests for params.go
 * By J. Stuart McMurray
 * Created 20260125
 * Last Modified 20260124
 */

import (
	"bytes"
	"testing"
	"text/template"
)

// Does Params.With work with?
func TestParamsWith(t *testing.T) {
	for n, c := range map[string]struct {
		tmpl string
		want string
	}{"empty": {
		tmpl: "",
		want: "",
	}, "nil_map": {
		tmpl: "{{.M.kittens}}",
		want: "<no value>",
	}, "one_with": {
		tmpl: `{{$a := .With "k1" "v1"}}{{$a.M.k1}}`,
		want: `v1`,
	}, "two_withs": {
		tmpl: `{{$a := .With "k1" "v1"}}{{$a = $a.With "k2" "v2"}}` +
			`{{$a.M.k1}} {{$a.M.k2}}`,
		want: `v1 v2`,
	}, "chained_with": {
		tmpl: `{{$a := (.With "k1" "v1").With "k2" "v2"}}` +
			`{{$a.M.k1}} {{$a.M.k2}}`,
		want: `v1 v2`,
	}} {
		t.Run(n, func(t *testing.T) {
			/* Roll a template. */
			tt, err := template.New("test_template").Parse(c.tmpl)
			if nil != err {
				t.Fatalf("Error parsing template: %s", err)
			}
			/* Try it. */
			buf := new(bytes.Buffer)
			if err := tt.Execute(buf, Params{}); nil != err {
				t.Fatalf("Template execution failed: %s", err)
			}
			/* Did it work? */
			if got, want := buf.String(), c.want; got != want {
				t.Errorf(
					"Template output incorrect\n"+
						"got:\n%s\n"+
						"want:\n%s",
					got,
					want,
				)
			}
		})
	}
}
