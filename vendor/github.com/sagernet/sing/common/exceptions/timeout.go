package exceptions

type TimeoutError interface {
	Timeout() bool
}

func IsTimeout(err error) bool {
	if timeoutErr, isTimeout := Cast[TimeoutError](err); isTimeout {
		return timeoutErr.Timeout()
	}
	return false
}
