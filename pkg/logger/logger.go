package logger

import (
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	logging *zap.Logger
)

func InitLogger(debugLog bool, logCaller bool, logStacktrace bool) {
	var level zapcore.Level
	var callerKey string
	var stacktraceKey string

	if debugLog {
		level = zapcore.DebugLevel
	} else {
		level = zapcore.InfoLevel
	}

	if logCaller {
		callerKey = "caller"
	}

	if logStacktrace {
		stacktraceKey = "stacktrace"
	}

	conf := zap.Config{
		Level:       zap.NewAtomicLevelAt(level),
		Development: false,
		Encoding:    "console",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "t",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      callerKey,
			MessageKey:     "msg",
			StacktraceKey:  stacktraceKey,
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	var err error
	logging, err = conf.Build()
	if err != nil {
		panic(err)
	}
}

func Errorf(format string, args ...interface{}) {
	logging.Error(fmt.Sprintf(format, args...))
}

func Infof(format string, args ...interface{}) {
	logging.Info(fmt.Sprintf(format, args...))
}

func Debugf(format string, args ...interface{}) {
	logging.Debug(fmt.Sprintf(format, args...))
}

func Warnf(format string, args ...interface{}) {
	logging.Warn(fmt.Sprintf(format, args...))
}

func Fatalf(format string, args ...interface{}) {
	logging.Fatal(fmt.Sprintf(format, args...))
}
