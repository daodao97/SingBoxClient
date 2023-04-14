package exceptions

import "net"

type TimeoutError interface {
	Timeout() bool
}

func IsTimeout(err error) bool {
	if netErr, isNetErr := err.(net.Error); isNetErr {
		//goland:noinspection GoDeprecation
		//nolint:staticcheck
		return netErr.Temporary() && netErr.Timeout()
	} else if timeoutErr, isTimeout := Cast[TimeoutError](err); isTimeout {
		return timeoutErr.Timeout()
	}
	return false
}
