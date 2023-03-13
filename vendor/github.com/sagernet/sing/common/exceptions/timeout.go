package exceptions

import (
	"errors"
	"os"
)

type TimeoutError interface {
	Timeout() bool
}

func IsTimeout(err error) bool {
	if unwrapErr := errors.Unwrap(err); unwrapErr != nil {
		err = unwrapErr
	}
	if ne, ok := err.(*os.SyscallError); ok {
		err = ne.Err
	}
	if timeoutErr, isTimeoutErr := err.(TimeoutError); isTimeoutErr {
		return timeoutErr.Timeout()
	}
	return false
}
