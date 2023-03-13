package golog

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"sort"

	"github.com/getlantern/hidden"
)

// TextOutput creates an output that writes text to different io.Writers for errors and debug
func TextOutput(errorWriter io.Writer, debugWriter io.Writer) Output {
	return &textOutput{
		E:  errorWriter,
		D:  debugWriter,
		pc: make([]uintptr, 10),
	}
}

type textOutput struct {
	// E is the error writer
	E io.Writer
	// D is the debug writer
	D  io.Writer
	pc []uintptr
}

func (o *textOutput) Error(prefix string, skipFrames int, printStack bool, severity string, arg interface{}, values map[string]interface{}) {
	o.print(o.E, prefix, skipFrames, printStack, severity, arg, values)
}

func (o *textOutput) Debug(prefix string, skipFrames int, printStack bool, severity string, arg interface{}, values map[string]interface{}) {
	o.print(o.D, prefix, skipFrames, printStack, severity, arg, values)
}

func (o *textOutput) print(writer io.Writer, prefix string, skipFrames int, printStack bool, severity string, arg interface{}, values map[string]interface{}) {
	buf := bufferPool.Get()
	defer bufferPool.Put(buf)

	GetPrepender()(buf)
	linePrefix := o.linePrefix(prefix, skipFrames)
	writeHeader := func() {
		buf.WriteString(severity)
		buf.WriteString(" ")
		buf.WriteString(linePrefix)
	}
	if arg != nil {
		ml, isMultiline := arg.(MultiLine)
		if !isMultiline {
			writeHeader()
			_, _ = fmt.Fprintf(buf, "%v", arg)
			printContext(buf, values)
			buf.WriteByte('\n')
		} else {
			mlp := ml.MultiLinePrinter()
			first := true
			for {
				writeHeader()
				more := mlp(buf)
				if first {
					printContext(buf, values)
					first = false
				}
				buf.WriteByte('\n')
				if !more {
					break
				}
			}
		}
	}
	b := []byte(hidden.Clean(buf.String()))
	_, err := writer.Write(b)
	if err != nil {
		errorOnLogging(err)
	}
	if printStack {
		if err := writeStack(writer, o.pc); err != nil {
			errorOnLogging(err)
		}
	}
}

// attaches the file and line number corresponding to
// the log message
func (o *textOutput) linePrefix(prefix string, skipFrames int) string {
	runtime.Callers(skipFrames, o.pc)
	funcForPc := runtime.FuncForPC(o.pc[0])
	file, line := funcForPc.FileLine(o.pc[0] - 1)
	return fmt.Sprintf("%s%s:%d ", prefix, filepath.Base(file), line)
}

func printContext(buf *bytes.Buffer, values map[string]interface{}) {
	if len(values) == 0 {
		return
	}
	buf.WriteString(" [")
	var keys []string
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for i, key := range keys {
		value := values[key]
		if i > 0 {
			buf.WriteString(" ")
		}
		buf.WriteString(key)
		buf.WriteString("=")
		_, _ = fmt.Fprintf(buf, "%v", value)
	}
	buf.WriteByte(']')
}
