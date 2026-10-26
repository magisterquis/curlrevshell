package crstemplate

/*
 * errors.go
 * Error types and values
 * By J. Stuart McMurray
 * Created 20261021
 * Last Modified 20261021
 */

import (
	"errors"
	"fmt"
)

// ErrNoSubtemplates indicates that a template file was specified which had no
// subtemplates.
var ErrNoSubtemplates = errors.New("no subtemplates")

// ErrOutOfIdeas indicates no C2 address was able to be derived from an HTTP
// request.
var ErrOutOfIdeas = errors.New("out of ideas")

// ErrOutsideSubtemplate indicates the file passed to Execute had data outside
// subtemplates; likely an old-style template.
var ErrOutsideSubtemplate = errors.New("non-subtemplate template data found")

// ErrRequestAlreadyAdded is returned by AddRequest if the Params to which
// a request was to be added already had one.
var ErrRequestAlreadyAdded = errors.New("Params already has a request")

// ExtraneousTemplateDataCheckError is returned by Execute when an error is
// encountered checking a template for extraneous (old-style) data.
type ExtraneousTemplateDataCheckError struct {
	Err error
}

// Error implements the error interface.
func (err ExtraneousTemplateDataCheckError) Error() string {
	return fmt.Sprintf("checking for extraneous template data: %v", err.Err)
}

// Unwrap returns err.Err.
func (err ExtraneousTemplateDataCheckError) Unwrap() error { return err.Err }
