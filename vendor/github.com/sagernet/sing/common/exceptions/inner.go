package exceptions

type HasInnerError interface {
	Unwrap() error
}

func Unwrap(err error) error {
	for {
		inner, ok := err.(HasInnerError)
		if !ok {
			break
		}
		innerErr := inner.Unwrap()
		if innerErr == nil {
			break
		}
		err = innerErr
	}
	return err
}
