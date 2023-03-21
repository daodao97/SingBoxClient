package exceptions

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"syscall"
	_ "unsafe"

	F "github.com/sagernet/sing/common/format"
)

type Handler interface {
	NewError(ctx context.Context, err error)
}

type MultiError interface {
	Unwrap() []error
}

func New(message ...any) error {
	return errors.New(F.ToString(message...))
}

func Cause(cause error, message ...any) error {
	return &causeError{F.ToString(message...), cause}
}

func Extend(cause error, message ...any) error {
	return &extendedError{F.ToString(message...), cause}
}

func IsClosedOrCanceled(err error) bool {
	return IsMulti(err, io.EOF, net.ErrClosed, io.ErrClosedPipe, os.ErrClosed, syscall.EPIPE, syscall.ECONNRESET, context.Canceled, context.DeadlineExceeded)
}

func IsClosed(err error) bool {
	return IsMulti(err, io.EOF, net.ErrClosed, io.ErrClosedPipe, os.ErrClosed, syscall.EPIPE, syscall.ECONNRESET)
}

func IsCanceled(err error) bool {
	return IsMulti(err, context.Canceled, context.DeadlineExceeded)
}
