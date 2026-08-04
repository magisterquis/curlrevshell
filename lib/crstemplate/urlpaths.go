package crstemplate

/*
 * urlpaths.go
 * Make sure URL paths work nicely
 * By J. Stuart McMurray
 * Created 20250215
 * Last Modified 20260803
 */

import (
	"reflect"
	"strings"
)

// DefaultURLPaths are the URL paths we use if we don't have any others.  This
// should not be modified.
var DefaultURLPaths = URLPaths{
	In:        DefaultURLPathIn,
	InOut:     DefaultURLPathInOut,
	Out:       DefaultURLPathOut,
	Websocket: DefaultURLPathWebsocket,
	Script:    DefaultURLPathScript,
}

// CleanURLPaths removes leading and trailing slashes from the fields in p.
// If an element of p is empty, it is taken from the corresponding field in
// DefaultURLPaths.
func CleanURLPaths(p *URLPaths) {
	v := reflect.ValueOf(p).Elem()
	for i := range v.NumField() {
		v.Field(i).SetString(strings.Trim(v.Field(i).String(), "/"))
		if v.Field(i).IsZero() {
			v.Field(i).Set(
				reflect.ValueOf(DefaultURLPaths).Field(i),
			)
		}
	}
}
