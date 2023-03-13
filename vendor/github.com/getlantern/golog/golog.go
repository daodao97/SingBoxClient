// Package golog implements logging functions that log errors to stderr and
// debug messages to stdout. Trace logging is also supported.
// Trace logs go to stdout as well, but they are only written if the program
// is run with environment variable "TRACE=true".
// A stack dump will be printed after the message if "PRINT_STACK=true".
package golog

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/getlantern/errors"
	"github.com/getlantern/hidden"
	"github.com/getlantern/ops"
	"github.com/oxtoacart/bpool"
)

const (
	// ERROR is an error Severity
	ERROR = 500

	// FATAL is an error Severity
	FATAL = 600
)

type outputFn func(prefix string, skipFrames int, printStack bool, severity string, arg interface{}, values map[string]interface{})

// Output is a log output that can optionally support structured logging
type Output interface {
	// Write debug messages
	Debug(prefix string, skipFrames int, printStack bool, severity string, arg interface{}, values map[string]interface{})

	// Write error messages
	Error(prefix string, skipFrames int, printStack bool, severity string, arg interface{}, values map[string]interface{})
}

var (
	output         Output
	outputMx       sync.RWMutex
	prepender      atomic.Value
	reporters      []ErrorReporter
	reportersMutex sync.RWMutex

	bufferPool = bpool.NewBufferPool(200)

	onFatal atomic.Value
)

// Severity is a level of error (higher values are more severe)
type Severity int

func (s Severity) String() string {
	switch s {
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

func init() {
	DefaultOnFatal()
	ResetOutputs()
	ResetPrepender()
}

// SetPrepender sets a function to write something, e.g., the timestamp, before
// each line of the log.
func SetPrepender(p func(io.Writer)) {
	prepender.Store(p)
}

func ResetPrepender() {
	SetPrepender(func(io.Writer) {})
}

func GetPrepender() func(io.Writer) {
	return prepender.Load().(func(io.Writer))
}

// SetOutputs sets the outputs for error and debug logs to use the given Outputs.
// Returns a function that resets outputs to their original values prior to calling SetOutputs.
// If env variable PRINT_JSON is set, use JSON output instead of plain text
func SetOutputs(errorOut io.Writer, debugOut io.Writer) (reset func()) {
	if printJson, _ := strconv.ParseBool(os.Getenv("PRINT_JSON")); printJson {
		return SetOutput(JsonOutput(errorOut, debugOut))
	}

	return SetOutput(TextOutput(errorOut, debugOut))
}

// SetOutput sets the Output to use for errors and debug messages
func SetOutput(out Output) (reset func()) {
	outputMx.Lock()
	defer outputMx.Unlock()
	oldOut := output
	output = out
	return func() {
		outputMx.Lock()
		defer outputMx.Unlock()
		output = oldOut
	}
}

// Deprecated: instead of calling ResetOutputs, use the reset function returned by SetOutputs.
func ResetOutputs() {
	SetOutputs(os.Stderr, os.Stdout)
}

func getErrorOut() outputFn {
	outputMx.RLock()
	defer outputMx.RUnlock()
	return output.Error
}

func getDebugOut() outputFn {
	outputMx.RLock()
	defer outputMx.RUnlock()
	return output.Debug
}

// RegisterReporter registers the given ErrorReporter. All logged Errors are
// sent to this reporter.
func RegisterReporter(reporter ErrorReporter) {
	reportersMutex.Lock()
	reporters = append(reporters, reporter)
	reportersMutex.Unlock()
}

// OnFatal configures golog to call the given function on any FATAL error. By
// default, golog calls os.Exit(1) on any FATAL error.
func OnFatal(fn func(err error)) {
	onFatal.Store(fn)
}

// DefaultOnFatal enables the default behavior for OnFatal
func DefaultOnFatal() {
	onFatal.Store(func(err error) {
		os.Exit(1)
	})
}

// MultiLine is an interface for arguments that support multi-line output.
type MultiLine interface {
	// MultiLinePrinter returns a function that can be used to print the
	// multi-line output. The returned function writes one line to the buffer and
	// returns true if there are more lines to write. This function does not need
	// to take care of trailing carriage returns, golog handles that
	// automatically.
	MultiLinePrinter() func(buf *bytes.Buffer) bool
}

// ErrorReporter is a function to which the logger will report errors.
// It the given error and corresponding message along with associated ops
// context. This should return quickly as it executes on the critical code
// path. The recommended approach is to buffer as much as possible and discard
// new reports if the buffer becomes saturated.
type ErrorReporter func(err error, severity Severity, ctx map[string]interface{})

type Logger interface {
	// Debug logs to stdout
	Debug(arg interface{})
	// Debugf logs to stdout
	Debugf(message string, args ...interface{})

	// Error logs to stderr
	Error(arg interface{}) error
	// Errorf logs to stderr. It returns the first argument that's an error, or
	// a new error built using fmt.Errorf if none of the arguments are errors.
	Errorf(message string, args ...interface{}) error

	// Fatal logs to stderr and then exits with status 1
	Fatal(arg interface{})
	// Fatalf logs to stderr and then exits with status 1
	Fatalf(message string, args ...interface{})

	// Trace logs to stderr only if TRACE=true
	Trace(arg interface{})
	// Tracef logs to stderr only if TRACE=true
	Tracef(message string, args ...interface{})

	// TraceOut provides access to an io.Writer to which trace information can
	// be streamed. If running with environment variable "TRACE=true", TraceOut
	// will point to os.Stderr, otherwise it will point to a ioutil.Discared.
	// Each line of trace information will be prefixed with this Logger's
	// prefix.
	TraceOut() io.Writer

	// IsTraceEnabled() indicates whether or not tracing is enabled for this
	// logger.
	IsTraceEnabled() bool

	// AsStdLogger returns an standard logger
	AsStdLogger() *log.Logger
}

func LoggerFor(prefix string) Logger {
	l := &logger{
		prefix: prefix + ": ",
	}

	trace := os.Getenv("TRACE")
	l.traceOn, _ = strconv.ParseBool(trace)
	if !l.traceOn {
		prefixes := strings.Split(trace, ",")
		for _, p := range prefixes {
			if prefix == strings.Trim(p, " ") {
				l.traceOn = true
				break
			}
		}
	}
	if l.traceOn {
		l.traceOut = l.newTraceWriter()
	} else {
		l.traceOut = ioutil.Discard
	}

	printStack := os.Getenv("PRINT_STACK")
	l.printStack, _ = strconv.ParseBool(printStack)

	return l
}

type logger struct {
	prefix     string
	traceOn    bool
	traceOut   io.Writer
	printStack bool
}

func (l *logger) print(write outputFn, skipFrames int, severity string, arg interface{}) {
	write(l.prefix, skipFrames+2, l.printStack, severity, arg, ops.AsMap(arg, false))
}

func (l *logger) printf(write outputFn, skipFrames int, severity string, message string, args ...interface{}) {
	l.print(write, skipFrames+1, severity, fmt.Sprintf(message, args...))
}

func (l *logger) Debug(arg interface{}) {
	l.print(getDebugOut(), 4, "DEBUG", arg)
}

func (l *logger) Debugf(message string, args ...interface{}) {
	l.printf(getDebugOut(), 4, "DEBUG", message, args...)
}

func (l *logger) Error(arg interface{}) error {
	return l.errorSkipFrames(arg, 1, ERROR)
}

func (l *logger) Errorf(message string, args ...interface{}) error {
	return l.errorSkipFrames(errors.NewOffset(1, message, args...), 1, ERROR)
}

func (l *logger) Fatal(arg interface{}) {
	fatal(l.errorSkipFrames(arg, 1, FATAL))
}

func (l *logger) Fatalf(message string, args ...interface{}) {
	fatal(l.errorSkipFrames(errors.NewOffset(1, message, args...), 1, FATAL))
}

func fatal(err error) {
	fn := onFatal.Load().(func(err error))
	fn(err)
}

func (l *logger) errorSkipFrames(arg interface{}, skipFrames int, severity Severity) error {
	var err error
	switch e := arg.(type) {
	case error:
		err = e
	default:
		err = fmt.Errorf("%v", e)
	}
	l.print(getErrorOut(), skipFrames+4, severity.String(), err)
	return report(err, severity)
}

func (l *logger) Trace(arg interface{}) {
	if l.traceOn {
		l.print(getDebugOut(), 4, "TRACE", arg)
	}
}

func (l *logger) Tracef(message string, args ...interface{}) {
	if l.traceOn {
		l.printf(getDebugOut(), 4, "TRACE", message, args...)
	}
}

func (l *logger) TraceOut() io.Writer {
	return l.traceOut
}

func (l *logger) IsTraceEnabled() bool {
	return l.traceOn
}

func (l *logger) newTraceWriter() io.Writer {
	pr, pw := io.Pipe()
	br := bufio.NewReader(pr)

	if !l.traceOn {
		return pw
	}
	go func() {
		defer func() {
			if err := pr.Close(); err != nil {
				errorOnLogging(err)
			}
		}()
		defer func() {
			if err := pw.Close(); err != nil {
				errorOnLogging(err)
			}
		}()

		for {
			line, err := br.ReadString('\n')
			if err == nil {
				// Log the line (minus the trailing newline)
				l.print(getDebugOut(), 6, "TRACE", line[:len(line)-1])
			} else {
				l.printf(getDebugOut(), 6, "TRACE", "TraceWriter closed due to unexpected error: %v", err)
				return
			}
		}
	}()

	return pw
}

type errorWriter struct {
	l *logger
}

// Write implements method of io.Writer, due to different call depth,
// it will not log correct file and line prefix
func (w *errorWriter) Write(p []byte) (n int, err error) {
	s := string(p)
	if s[len(s)-1] == '\n' {
		s = s[:len(s)-1]
	}
	w.l.print(getErrorOut(), 6, "ERROR", s)
	return len(p), nil
}

func (l *logger) AsStdLogger() *log.Logger {
	return log.New(&errorWriter{l}, "", 0)
}

func errorOnLogging(err error) {
	_, _ = fmt.Fprintf(os.Stderr, "Unable to log: %v\n", err)
}

func report(err error, severity Severity) error {
	var reportersCopy []ErrorReporter
	reportersMutex.RLock()
	if len(reporters) > 0 {
		reportersCopy = make([]ErrorReporter, len(reporters))
		copy(reportersCopy, reporters)
	}
	reportersMutex.RUnlock()

	if len(reportersCopy) > 0 {
		ctx := ops.AsMap(err, true)
		ctx["severity"] = severity.String()
		for _, reporter := range reportersCopy {
			// We include globals when reporting
			reporter(err, severity, ctx)
		}
	}
	return err
}

func writeStack(w io.Writer, pcs []uintptr) error {
	for _, pc := range pcs {
		funcForPc := runtime.FuncForPC(pc)
		if funcForPc == nil {
			break
		}
		name := funcForPc.Name()
		if strings.HasPrefix(name, "runtime.") {
			break
		}
		file, line := funcForPc.FileLine(pc)
		_, err := fmt.Fprintf(w, "\t%s\t%s: %d\n", name, file, line)
		if err != nil {
			return err
		}
	}

	return nil
}

func argToString(arg interface{}) string {
	if arg != nil {
		if ml, isMultiline := arg.(MultiLine); !isMultiline {
			return fmt.Sprintf("%v", arg)
		} else {
			buf := bufferPool.Get()
			defer bufferPool.Put(buf)
			mlp := ml.MultiLinePrinter()
			for {
				more := mlp(buf)
				buf.WriteByte('\n')
				if !more {
					break
				}
			}
			return hidden.Clean(buf.String())
		}
	}
	return ""
}
