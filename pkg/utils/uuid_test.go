package utils

import (
	"github.com/stretchr/testify/assert"
	"testing"
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
