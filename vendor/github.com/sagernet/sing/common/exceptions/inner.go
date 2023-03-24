package exceptions

import "github.com/sagernet/sing/common"

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

func Cast[T any](err error) (T, bool) {
	if err == nil {
		return common.DefaultValue[T](), false
	}

	for {
		interfaceError, isInterface := err.(T)
		if isInterface {
			return interfaceError, true
		}
		switch x := err.(type) {
		case interface{ Unwrap() error }:
			err = x.Unwrap()
			if err == nil {
				return common.DefaultValue[T](), false
			}
		case interface{ Unwrap() []error }:
			for _, innerErr := range x.Unwrap() {
				if interfaceError, isInterface = Cast[T](innerErr); isInterface {
					return interfaceError, true
				}
			}
			return common.DefaultValue[T](), false
		default:
			return common.DefaultValue[T](), false
		}
	}
}
