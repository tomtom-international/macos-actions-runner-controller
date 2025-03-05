package utils

import (
	"github.com/google/uuid"
)

type UID string

func NewUUID() UID {
	return UID(uuid.New().String())
}

func NewShortUUID() UID {
	return UID(uuid.New().String()[:8])
}

func (u UID) Short() UID {
	return u[:8]
}
