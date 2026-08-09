package crsadapter

/*
 * errors.go
 * Error types and values
 * By J. Stuart McMurray
 * Created 20260809
 * Last Modified 20260809
 */

// ConnResponseError is an error type wrapping a ConnResponse.
type ConnResponseError struct {
	ConnResponse
}

// Error returns err's underlying string, which may be the empty string.
func (err ConnResponseError) Error() string { return err.ConnResponse.Error }

// Is indicates whether err matches target.  If target is nil and err's
// underlying string is nil, Is returns true.  In general, it is preferable to
// use [ConnResponse.ToError] than wrap ConnResponse in a ConnResponseError
// directly.
func (err ConnResponseError) Is(target error) bool {
	if nil == target && "" == err.ConnResponse.Error {
		return true
	}
	cr, ok := target.(ConnResponseError)
	return ok && err.ConnResponse.Error == cr.ConnResponse.Error
}
