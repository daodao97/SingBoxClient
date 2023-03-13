package golog

import (
	"go.uber.org/zap"
)

// ZapOutput creates an output that writes using the provided Zap logger
func ZapOutput(zapLogger *zap.Logger) Output {
	return &zapOutput{zapLogger}
}

type zapOutput struct {
	*zap.Logger
}

func (o *zapOutput) Error(prefix string, skipFrames int, printStack bool, severity string, arg interface{}, values map[string]interface{}) {
	fields, configuredLogger := prepareLogger(prefix, values, o, skipFrames)
	configuredLogger.Error(argToString(arg), fields...)
}

func prepareLogger(prefix string, values map[string]interface{}, o *zapOutput, skipFrames int) ([]zap.Field, *zap.Logger) {
	// prefix contains ': ' at the end, strip it
	cleanPrefix := prefix[0 : len(prefix)-2]
	fields := []zap.Field{}
	for k, v := range values {
		fields = append(fields, zap.Any(k, v))
	}

	return fields, o.Logger.Named(cleanPrefix).WithOptions(zap.AddCaller(), zap.AddStacktrace(zap.ErrorLevel), zap.AddCallerSkip(skipFrames-3))
}

func (o *zapOutput) Debug(prefix string, skipFrames int, printStack bool, severity string, arg interface{}, values map[string]interface{}) {
	fields, configuredLogger := prepareLogger(prefix, values, o, skipFrames)
	configuredLogger.Debug(argToString(arg), fields...)

}
