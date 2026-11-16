/*
 * Copyright 2025 TomTom N.V.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package logger

import (
	"fmt"

	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger/internal/current"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func InitLogger(debugLog, logCaller, logStacktrace, jsonLog bool) error {
	var level zapcore.Level

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

	l, err := conf.Build()
	if err != nil {
		return err
	}
	current.Logger.Store(l)
	return nil
}

func Errorf(format string, args ...interface{}) {
	current.Logger.Load().Error(fmt.Sprintf(format, args...))
}

func Infof(format string, args ...interface{}) {
	current.Logger.Load().Info(fmt.Sprintf(format, args...))
}

func Debugf(format string, args ...interface{}) {
	current.Logger.Load().Debug(fmt.Sprintf(format, args...))
}

func Warnf(format string, args ...interface{}) {
	current.Logger.Load().Warn(fmt.Sprintf(format, args...))
}

func Fatalf(format string, args ...interface{}) {
	current.Logger.Load().Fatal(fmt.Sprintf(format, args...))
}

func Sync() {
	_ = current.Logger.Load().Sync()
}
