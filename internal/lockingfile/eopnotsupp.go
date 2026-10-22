//go:build !darwin

package lockingfile

/*
 * eopnotsupp.go
 * Does an error indicate an operation is not supported?
 * By J. Stuart McMurray
 * Created 20261020
 * Last Modified 20261020
 */

import (
	"errors"
	"syscall"
)

// isNotSupported indicates if err is or wraps [syscall.EOPNOTSUPP].
func isNotSupported(err error) bool {
	return errors.Is(err, syscall.EOPNOTSUPP)
}
