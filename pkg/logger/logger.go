package logger

import (
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	log *zap.Logger
)

func InitLogger(debugLog, logCaller, logStacktrace, jsonLog bool) error {
	var level zapcore.Level
	var err error

	conf := zap.Config{
		Level:       zap.NewAtomicLevelAt(level),
		Development: false,
		Encoding:    "console",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "t",
			LevelKey:       "level",
			NameKey:        "logger",
			MessageKey:     "msg",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	if debugLog {
		level = zapcore.DebugLevel
	} else {
		level = zapcore.InfoLevel
	}
	if jsonLog {
		conf.Encoding = "json"
	} else {
		conf.Encoding = "console"
	}
	if logCaller {
		conf.EncoderConfig.CallerKey = "caller"
	}
	if logStacktrace {
		conf.EncoderConfig.StacktraceKey = "stacktrace"
	}
	conf.Level = zap.NewAtomicLevelAt(level)

	log, err = conf.Build()
	if err != nil {
		return err
	}
	return nil
}

func Errorf(format string, args ...interface{}) {
	log.Error(fmt.Sprintf(format, args...))
}

func Infof(format string, args ...interface{}) {
	log.Info(fmt.Sprintf(format, args...))
}

func Debugf(format string, args ...interface{}) {
	log.Debug(fmt.Sprintf(format, args...))
}

func Warnf(format string, args ...interface{}) {
	log.Warn(fmt.Sprintf(format, args...))
}

func Fatalf(format string, args ...interface{}) {
	log.Fatal(fmt.Sprintf(format, args...))
}

func Sync() {
	if log != nil {
		_ = log.Sync()
	}
}
