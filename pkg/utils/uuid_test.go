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

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewUUID(t *testing.T) {
	uuid := NewUUID()
	assert.Equal(t, 36, len(uuid), "Expected UUID length of 36")
}

func TestNewShortUUID(t *testing.T) {
	shortUUID := NewShortUUID()
	assert.Equal(t, 8, len(shortUUID), "Expected ShortUUID length of 8")
}

func TestUID_Short(t *testing.T) {
	uuid := UID("01234567-89ab-cdef-0123-456789abcdef")
	shortUUID := uuid.Short()
	assert.Equal(t, "01234567", string(shortUUID), "Expected shortUUID=01234567, got %s", shortUUID)
}
