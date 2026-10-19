package tmplfuncs

/*
 * errors.go
 * Error types and values
 * By J. Stuart McMurray
 * Created 20261019
 * Last Modified 20261019
 */

import (
	"fmt"
)

// errMissingValue is returned from [Map] to indicate an odd number of
// arguments was passed.
var errMissingValue = fmt.Errorf("need a value for each key")

// incorrectKeyTypeError is returned from [Map] to indicate a non-string key.
type incorrectKeyTypeError struct {
	key any
}

// Error implements the Error interface.
func (err incorrectKeyTypeError) Error() string {
	return fmt.Sprintf("key %v is a %T, not a string", err.key, err.key)
}
