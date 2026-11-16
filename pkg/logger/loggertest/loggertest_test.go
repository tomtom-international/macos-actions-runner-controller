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

package loggertest_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger/loggertest"
	"go.uber.org/zap/zapcore"
)

func TestObserveCapturesEntriesAtLevel(t *testing.T) {
	logs := loggertest.Observe(t, zapcore.InfoLevel)

	logger.Debugf("hidden")
	logger.Infof("hello %d", 1)

	require.Equal(t, 1, logs.Len())
	assert.Equal(t, "hello 1", logs.All()[0].Message)
	assert.Equal(t, zapcore.InfoLevel, logs.All()[0].Level)
}

func TestObserveRestoresPreviousLoggerOnCleanup(t *testing.T) {
	outer := loggertest.Observe(t, zapcore.DebugLevel)

	t.Run("inner", func(t *testing.T) {
		loggertest.Observe(t, zapcore.DebugLevel)
		logger.Infof("inner")
	})
	logger.Infof("outer")

	require.Equal(t, 1, outer.Len())
	assert.Equal(t, "outer", outer.All()[0].Message)
}
