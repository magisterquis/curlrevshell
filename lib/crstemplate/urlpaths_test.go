package crstemplate

/*
 * urlpaths_test.go
 * Tests for urlpaths.go
 * By J. Stuart McMurray
 * Created 20250215
 * Last Modified 20260805
 */

import (
	"reflect"
	"testing"
)

// Make sure unset fields are set.
func TestCleanURLPaths_UnsetField(t *testing.T) {
	var (
		full = reflect.ValueOf(URLPaths{
			In:     "dummyIn",
			InOut:  "dummyInOut",
			Out:    "dummyOut",
			Script: "dummyScript",
		})
		defUPV = reflect.ValueOf(DefaultURLPaths)
		cases  = make([]reflect.Value, full.NumField())
	)
	/* One test case for each missing field. */
	for fi := range full.NumField() {
		have := reflect.ValueOf(new(URLPaths))
		have.Elem().Set(full)
		have.Elem().Field(fi).SetString("")
		cases[fi] = have
	}
	for i, have := range cases {
		ufn := full.Type().Field(i).Name
		t.Run("unset_"+ufn, func(t *testing.T) {
			/* Make set the missing field. */
			p, ok := have.Interface().(*URLPaths)
			if !ok {
				t.Fatalf(
					"Interface was %T not %T",
					have.Interface(),
					p,
				)
			}
			CleanURLPaths(p)
			/* Make sure it's set and the rest aren't. */
			for j := range full.NumField() {
				var (
					got = have.Elem().Field(
						j,
					).Interface().(string)
					want string
				)
				/* Work out what it should be. */
				if i == j { /* Looking at the empty field */
					want = defUPV.Field(
						j,
					).Interface().(string)
				} else {
					want = full.Field(
						j,
					).Interface().(string)
				}
				if got != want {
					t.Errorf(
						"Field %s incorrect:\n"+
							"  got: %s\n"+
							" want: %s",
						have.Elem().Type().Field(
							j,
						).Name,
						got,
						want,
					)
				}
			}
		})
	}
}

// Make sure slash-removal works.
func TestCleanURLPaths_Trim(t *testing.T) {
	/* Make sure slash-removal works. */
	t.Run("remove_slashes", func(t *testing.T) {
		have := URLPaths{
			In:     "in/",
			InOut:  "/in_out",
			Out:    "out/",
			Script: "/////script/////",
		}
		want := URLPaths{
			In:     "in",
			InOut:  "in_out",
			Out:    "out",
			Script: "script",
		}
		got := have
		CleanURLPaths(&got)
		if got != want {
			t.Errorf(
				"Incorrect cleaned paths:\n"+
					" got: %#v\n"+
					"want: %#v",
				got,
				want,
			)
		}
	})

	/* Make sure a field with just slashes is treated as empty. */
	t.Run("only_slashes", func(t *testing.T) {
		got := URLPaths{
			In:     "///",
			InOut:  "///",
			Out:    "///",
			Script: "///",
		}
		if n := firstEmptyField(got, ""); "" != n {
			t.Fatalf("Empty field %s", n)
		}
		CleanURLPaths(&got)
		if want := DefaultURLPaths; got != want {
			t.Fatalf(
				"Slashes not replaced by defaults:\n"+
					" got: %#v\n"+
					"want: %#v",
				got,
				want,
			)
		}
	})

	/* Make sure emptiness turns into the defaults. */
	t.Run("empty", func(t *testing.T) {
		var got URLPaths
		CleanURLPaths(&got)
		if want := DefaultURLPaths; got != want {
			t.Fatalf(
				"Empty params not turned into DefaultParams:"+
					" got: %#v\n"+
					"want: %#v",
				got,
				want,
			)
		}
	})
}
