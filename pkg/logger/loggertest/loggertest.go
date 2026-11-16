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

// Package loggertest provides test helpers for package logger.
package loggertest

import (
	"testing"

	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger/internal/current"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// Observe routes package logger output at or above level into the returned logs until t ends.
func Observe(t testing.TB, level zapcore.Level) *observer.ObservedLogs {
	t.Helper()
	obsCore, logs := observer.New(level)
	prev := current.Logger.Swap(zap.New(obsCore))
	t.Cleanup(func() { current.Logger.Store(prev) })
	return logs
}
