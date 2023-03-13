/*
Package errors defines error types used across Lantern project.

	n, err := Foo()
	if err != nil {
	    return n, errors.New("Unable to do Foo: %v", err)
	}

or

  n, err := Foo()
	return n, errors.Wrap(err)

New() method will create a new error with err as its cause. Wrap will wrap err,
returning nil if err is nil.  If err is an error from Go's standard library,
errors will extract details from that error, at least the Go type name and the
return value of err.Error().

One can record the operation on which the error occurred using Op():

  return n, errors.New("Unable to do Foo: %v", err).Op("FooDooer")

One can also record additional data:

  return n, errors.
		New("Unable to do Foo: %v", err).
		Op("FooDooer").
		With("mydata", "myvalue").
		With("moredata", 5)

When used with github.com/getlantern/ops, Error captures its current context
and propagates that data for use in calling layers.

When used with github.com/getlantern/golog, Error provides stacktraces:

	Hello World
		at github.com/getlantern/errors.TestNewWithCause (errors_test.go:999)
		at testing.tRunner (testing.go:999)
		at runtime.goexit (asm_amd999.s:999)
	Caused by: World
		at github.com/getlantern/errors.buildCause (errors_test.go:999)
		at github.com/getlantern/errors.TestNewWithCause (errors_test.go:999)
		at testing.tRunner (testing.go:999)
		at runtime.goexit (asm_amd999.s:999)
	Caused by: orld
	Caused by: ld
		at github.com/getlantern/errors.buildSubSubCause (errors_test.go:999)
		at github.com/getlantern/errors.buildSubCause (errors_test.go:999)
		at github.com/getlantern/errors.buildCause (errors_test.go:999)
		at github.com/getlantern/errors.TestNewWithCause (errors_test.go:999)
		at testing.tRunner (testing.go:999)
		at runtime.goexit (asm_amd999.s:999)
	Caused by: d

It's the caller's responsibility to avoid race conditions accessing the same
error instance from multiple goroutines.
*/
package errors

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/getlantern/context"
	"github.com/getlantern/hidden"
	"github.com/getlantern/ops"
	"github.com/go-stack/stack"
)

// Error wraps system and application defined errors in unified structure for
// reporting and logging. It's not meant to be created directly. User New(),
// Wrap() and Report() instead.
type Error interface {
	error
	context.Contextual

	// ErrorClean returns a non-parameterized version of the error whenever
	// possible. For example, if the error text is:
	//
	//     unable to dial www.google.com caused by: i/o timeout
	//
	// ErrorClean might return:
	//
	//     unable to dial %v caused by: %v
	//
	// This can be useful when performing analytics on the error.
	ErrorClean() string

	// MultiLinePrinter implements the interface golog.MultiLine
	MultiLinePrinter() func(buf *bytes.Buffer) bool

	// Op attaches a hint of the operation triggers this Error. Many error types
	// returned by net and os package have Op pre-filled.
	Op(op string) Error

	// With attaches arbitrary field to the error. keys will be normalized as
	// underscore_divided_words, so all characters except letters and numbers will
	// be replaced with underscores, and all letters will be lowercased.
	With(key string, value interface{}) Error

	// RootCause returns the bottom-most cause of this Error. If the Error
	// resulted from wrapping a plain error, the wrapped error will be returned as
	// the cause.
	RootCause() error
}

type baseError struct {
	errID     uint64
	hiddenID  string
	data      context.Map
	context   context.Map
	callStack stack.CallStack
}

// New creates an Error with supplied description and format arguments to the
// description. If any of the arguments is an error, we use that as the cause.
func New(desc string, args ...interface{}) Error {
	return NewOffset(1, desc, args...)
}

// NewOffset is like New but offsets the stack by the given offset. This is
// useful for utilities like golog that may create errors on behalf of others.
func NewOffset(offset int, desc string, args ...interface{}) Error {
	e := buildError(desc, fmt.Sprintf(desc, args...))
	e.attachStack(2 + offset)
	for _, arg := range args {
		wrapped, isError := arg.(error)
		if isError {
			op, _, _, extraData := parseError(wrapped)
			if op != "" {
				e.Op(op)
			}
			for k, v := range extraData {
				e.data[k] = v
			}
			we := &wrappingError{e, wrapped}
			bufferError(we)
			return we
		}
	}
	bufferError(e)
	return e
}

// Wrap creates an Error based on the information in an error instance.  It
// returns nil if the error passed in is nil, so we can simply call
// errors.Wrap(s.l.Close()) regardless there's an error or not. If the error is
// already wrapped, it is returned as is.
func Wrap(err error) Error {
	if err == nil {
		return nil
	}
	if e, ok := err.(Error); ok {
		return e
	}

	op, goType, desc, extraData := parseError(err)
	if desc == "" {
		desc = err.Error()
	}
	e := buildError(desc, desc)
	e.attachStack(2)
	if op != "" {
		e.Op(op)
	}
	e.data["error_type"] = goType
	for k, v := range extraData {
		e.data[k] = v
	}
	if cause := getCause(err); cause != nil {
		we := &wrappingError{e, cause}
		bufferError(we)
		return we
	}
	bufferError(e)
	return e
}

// Fill implements the method from the context.Contextual interface.
func (e *baseError) Fill(m context.Map) {
	if e == nil {
		return
	}

	// Include the context, which supercedes the cause
	for key, value := range e.context {
		m[key] = value
	}
	// Now include the error's data, which supercedes everything
	for key, value := range e.data {
		m[key] = value
	}
}

func (e *baseError) Op(op string) Error {
	e.data["error_op"] = op
	return e
}

func (e *baseError) With(key string, value interface{}) Error {
	parts := strings.FieldsFunc(key, func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsNumber(c)
	})
	k := strings.ToLower(strings.Join(parts, "_"))
	if k == "error" || k == "error_op" {
		// Never overwrite these
		return e
	}
	switch actual := value.(type) {
	case string, int, bool, time.Time:
		e.data[k] = actual
	default:
		e.data[k] = fmt.Sprint(actual)
	}
	return e
}

func (e *baseError) RootCause() error {
	return e
}

func (e *baseError) ErrorClean() string {
	return e.data["error"].(string)
}

// Error satisfies the error interface
func (e *baseError) Error() string {
	return e.data["error_text"].(string) + e.hiddenID
}

func (e *baseError) MultiLinePrinter() func(*bytes.Buffer) bool {
	return e.topLevelPrinter()
}

func (e *baseError) topLevelPrinter() func(*bytes.Buffer) bool {
	printingStack := false
	stackPosition := 0
	return func(buf *bytes.Buffer) bool {
		if !printingStack {
			buf.WriteString(e.Error())
			printingStack = true
			return len(e.callStack) > 0
		}
		call := e.callStack[stackPosition]
		fmt.Fprintf(buf, "  at %+n (%s:%d)", call, call, call)
		stackPosition++
		return stackPosition < len(e.callStack)
	}
}

func (e *baseError) attachStack(skip int) {
	call := stack.Caller(skip)
	e.callStack = stack.Trace().TrimBelow(call)
	e.data["error_location"] = fmt.Sprintf("%+n (%s:%d)", call, call, call)
}

func (e *baseError) id() uint64 {
	return e.errID
}

func (e *baseError) setID(id uint64) {
	e.errID = id
}

func (e *baseError) setHiddenID(id string) {
	e.hiddenID = id
}

func buildError(desc string, fullText string) *baseError {
	e := &baseError{
		data: make(context.Map),
		// We capture the current context to allow it to propagate to higher layers.
		context: ops.AsMap(nil, false),
	}

	cleanedDesc := hidden.Clean(desc)
	e.data["error"] = cleanedDesc
	if fullText != "" {
		e.data["error_text"] = hidden.Clean(fullText)
	} else {
		e.data["error_text"] = cleanedDesc
	}
	e.data["error_type"] = "errors.Error"

	return e
}

type topLevelPrinter interface {
	// Returns a printer which prints only the top-level error and any associated stack trace. The
	// output of this printer will be a prefix of the output from MultiLinePrinter().
	topLevelPrinter() func(*bytes.Buffer) bool
}

type unwrapper interface {
	Unwrap() error
}

type wrappingError struct {
	*baseError
	wrapped error
}

// Implements error unwrapping as described in the standard library's errors package:
// https://golang.org/pkg/errors/#pkg-overview
func (e *wrappingError) Unwrap() error {
	return e.wrapped
}

func (e *wrappingError) Fill(m context.Map) {
	type filler interface{ Fill(context.Map) }

	applyToChain(e.wrapped, func(err error) {
		if f, ok := err.(filler); ok {
			f.Fill(m)
		}
	})
	e.baseError.Fill(m)
}

func (e *wrappingError) RootCause() error {
	return unwrapToRoot(e)
}

func (e *wrappingError) MultiLinePrinter() func(*bytes.Buffer) bool {
	var (
		currentPrinter = e.baseError.topLevelPrinter()
		nextErr        = e.wrapped
		prefix         = ""
	)
	return func(buf *bytes.Buffer) bool {
		fmt.Fprint(buf, prefix)
		if currentPrinter(buf) {
			prefix = ""
			return true
		}
		if nextErr == nil {
			return false
		}
		currentPrinter = getTopLevelPrinter(nextErr)
		prefix = "Caused by: "
		if uw, ok := nextErr.(unwrapper); ok {
			nextErr = uw.Unwrap()
		} else {
			nextErr = nil
		}
		return true
	}
}

// We have to implement these two methods or the fluid syntax will result in the embedded *baseError
// being returned, not the *wrappingError.

func (e *wrappingError) Op(op string) Error {
	e.baseError = e.baseError.Op(op).(*baseError)
	return e
}

func (e *wrappingError) With(key string, value interface{}) Error {
	e.baseError = e.baseError.With(key, value).(*baseError)
	return e
}

func getTopLevelPrinter(err error) func(*bytes.Buffer) bool {
	if tlp, ok := err.(topLevelPrinter); ok {
		return tlp.topLevelPrinter()
	}
	return func(buf *bytes.Buffer) bool {
		fmt.Fprint(buf, err)
		return false
	}
}

func getCause(e error) error {
	if uw, ok := e.(unwrapper); ok {
		return uw.Unwrap()
	}
	// Look for hidden *baseErrors
	hiddenIDs, extractErr := hidden.Extract(e.Error())
	if extractErr == nil && len(hiddenIDs) > 0 {
		// Take the first hidden ID as our cause
		return get(hiddenIDs[0])
	}
	return nil
}

func unwrapToRoot(e error) error {
	if uw, ok := e.(unwrapper); ok {
		return unwrapToRoot(uw.Unwrap())
	}
	return e
}

// Applies f to the chain of errors unwrapped from err. The function is applied to the root cause
// first and err last.
func applyToChain(err error, f func(error)) {
	if uw, ok := err.(unwrapper); ok {
		applyToChain(uw.Unwrap(), f)
	}
	f(err)
}

func parseError(err error) (op string, goType string, desc string, extra map[string]string) {
	extra = make(map[string]string)

	// interfaces
	if _, ok := err.(net.Error); ok {
		if opError, ok := err.(*net.OpError); ok {
			op = opError.Op
			if opError.Source != nil {
				extra["remote_addr"] = opError.Source.String()
			}
			if opError.Addr != nil {
				extra["local_addr"] = opError.Addr.String()
			}
			extra["network"] = opError.Net
			err = opError.Err
		}
		switch actual := err.(type) {
		case *net.AddrError:
			goType = "net.AddrError"
			desc = actual.Err
			extra["addr"] = actual.Addr
		case *net.DNSError:
			goType = "net.DNSError"
			desc = actual.Err
			extra["domain"] = actual.Name
			if actual.Server != "" {
				extra["dns_server"] = actual.Server
			}
		case *net.InvalidAddrError:
			goType = "net.InvalidAddrError"
			desc = actual.Error()
		case *net.ParseError:
			goType = "net.ParseError"
			desc = "invalid " + actual.Type
			extra["text_to_parse"] = actual.Text
		case net.UnknownNetworkError:
			goType = "net.UnknownNetworkError"
			desc = "unknown network"
		case syscall.Errno:
			goType = "syscall.Errno"
			desc = actual.Error()
		case *url.Error:
			goType = "url.Error"
			desc = actual.Err.Error()
			op = actual.Op
		default:
			goType = reflect.TypeOf(err).String()
			desc = err.Error()
		}
		return
	}
	if _, ok := err.(runtime.Error); ok {
		desc = err.Error()
		switch err.(type) {
		case *runtime.TypeAssertionError:
			goType = "runtime.TypeAssertionError"
		default:
			goType = reflect.TypeOf(err).String()
		}
		return
	}

	// structs
	switch actual := err.(type) {
	case *http.ProtocolError:
		desc = actual.ErrorString
		if name, ok := httpProtocolErrors[err]; ok {
			goType = name
		} else {
			goType = "http.ProtocolError"
		}
	case url.EscapeError, *url.EscapeError:
		goType = "url.EscapeError"
		desc = "invalid URL escape"
	case url.InvalidHostError, *url.InvalidHostError:
		goType = "url.InvalidHostError"
		desc = "invalid character in host name"
	case *textproto.Error:
		goType = "textproto.Error"
		desc = actual.Error()
	case textproto.ProtocolError, *textproto.ProtocolError:
		goType = "textproto.ProtocolError"
		desc = actual.Error()

	case tls.RecordHeaderError:
		goType = "tls.RecordHeaderError"
		desc = actual.Msg
		extra["header"] = hex.EncodeToString(actual.RecordHeader[:])
	case x509.CertificateInvalidError:
		goType = "x509.CertificateInvalidError"
		desc = actual.Error()
	case x509.ConstraintViolationError:
		goType = "x509.ConstraintViolationError"
		desc = actual.Error()
	case x509.HostnameError:
		goType = "x509.HostnameError"
		desc = actual.Error()
		extra["host"] = actual.Host
	case x509.InsecureAlgorithmError:
		goType = "x509.InsecureAlgorithmError"
		desc = actual.Error()
	case x509.SystemRootsError:
		goType = "x509.SystemRootsError"
		desc = actual.Error()
	case x509.UnhandledCriticalExtension:
		goType = "x509.UnhandledCriticalExtension"
		desc = actual.Error()
	case x509.UnknownAuthorityError:
		goType = "x509.UnknownAuthorityError"
		desc = actual.Error()
	case hex.InvalidByteError:
		goType = "hex.InvalidByteError"
		desc = "invalid byte"
	case *json.InvalidUTF8Error:
		goType = "json.InvalidUTF8Error"
		desc = "invalid UTF-8 in string"
	case *json.InvalidUnmarshalError:
		goType = "json.InvalidUnmarshalError"
		desc = actual.Error()
	case *json.MarshalerError:
		goType = "json.MarshalerError"
		desc = actual.Error()
	case *json.SyntaxError:
		goType = "json.SyntaxError"
		desc = actual.Error()
	case *json.UnmarshalFieldError:
		goType = "json.UnmarshalFieldError"
		desc = actual.Error()
	case *json.UnmarshalTypeError:
		goType = "json.UnmarshalTypeError"
		desc = actual.Error()
	case *json.UnsupportedTypeError:
		goType = "json.UnsupportedTypeError"
		desc = actual.Error()
	case *json.UnsupportedValueError:
		goType = "json.UnsupportedValueError"
		desc = actual.Error()

	case *os.LinkError:
		goType = "os.LinkError"
		desc = actual.Error()
	case *os.PathError:
		goType = "os.PathError"
		op = actual.Op
		desc = actual.Err.Error()
	case *os.SyscallError:
		goType = "os.SyscallError"
		op = actual.Syscall
		desc = actual.Err.Error()
	case *exec.Error:
		goType = "exec.Error"
		desc = actual.Err.Error()
	case *exec.ExitError:
		goType = "exec.ExitError"
		desc = actual.Error()
		// TODO: limit the length
		extra["stderr"] = string(actual.Stderr)
	case *strconv.NumError:
		goType = "strconv.NumError"
		desc = actual.Err.Error()
		extra["function"] = actual.Func
	case *time.ParseError:
		goType = "time.ParseError"
		desc = actual.Message
	default:
		desc = err.Error()
		if t, ok := miscErrors[err]; ok {
			goType = t
			return
		}
		goType = reflect.TypeOf(err).String()
	}
	return
}

var httpProtocolErrors = map[error]string{
	http.ErrHeaderTooLong:        "http.ErrHeaderTooLong",
	http.ErrShortBody:            "http.ErrShortBody",
	http.ErrNotSupported:         "http.ErrNotSupported",
	http.ErrUnexpectedTrailer:    "http.ErrUnexpectedTrailer",
	http.ErrMissingContentLength: "http.ErrMissingContentLength",
	http.ErrNotMultipart:         "http.ErrNotMultipart",
	http.ErrMissingBoundary:      "http.ErrMissingBoundary",
}

var miscErrors = map[error]string{
	bufio.ErrInvalidUnreadByte: "bufio.ErrInvalidUnreadByte",
	bufio.ErrInvalidUnreadRune: "bufio.ErrInvalidUnreadRune",
	bufio.ErrBufferFull:        "bufio.ErrBufferFull",
	bufio.ErrNegativeCount:     "bufio.ErrNegativeCount",
	bufio.ErrTooLong:           "bufio.ErrTooLong",
	bufio.ErrNegativeAdvance:   "bufio.ErrNegativeAdvance",
	bufio.ErrAdvanceTooFar:     "bufio.ErrAdvanceTooFar",
	bufio.ErrFinalToken:        "bufio.ErrFinalToken",

	http.ErrWriteAfterFlush:    "http.ErrWriteAfterFlush",
	http.ErrBodyNotAllowed:     "http.ErrBodyNotAllowed",
	http.ErrHijacked:           "http.ErrHijacked",
	http.ErrContentLength:      "http.ErrContentLength",
	http.ErrBodyReadAfterClose: "http.ErrBodyReadAfterClose",
	http.ErrHandlerTimeout:     "http.ErrHandlerTimeout",
	http.ErrLineTooLong:        "http.ErrLineTooLong",
	http.ErrMissingFile:        "http.ErrMissingFile",
	http.ErrNoCookie:           "http.ErrNoCookie",
	http.ErrNoLocation:         "http.ErrNoLocation",
	http.ErrSkipAltProtocol:    "http.ErrSkipAltProtocol",

	io.EOF:              "io.EOF",
	io.ErrClosedPipe:    "io.ErrClosedPipe",
	io.ErrNoProgress:    "io.ErrNoProgress",
	io.ErrShortBuffer:   "io.ErrShortBuffer",
	io.ErrShortWrite:    "io.ErrShortWrite",
	io.ErrUnexpectedEOF: "io.ErrUnexpectedEOF",

	os.ErrInvalid:    "os.ErrInvalid",
	os.ErrPermission: "os.ErrPermission",
	os.ErrExist:      "os.ErrExist",
	os.ErrNotExist:   "os.ErrNotExist",

	exec.ErrNotFound: "exec.ErrNotFound",

	x509.ErrUnsupportedAlgorithm: "x509.ErrUnsupportedAlgorithm",
	x509.IncorrectPasswordError:  "x509.IncorrectPasswordError",

	hex.ErrLength: "hex.ErrLength",
}
