package platform

import "github.com/google/uuid"

type IDGenerator interface {
	New(prefix string) string
}

type UUIDGenerator struct{}

func (UUIDGenerator) New(prefix string) string {
	id := uuid.NewString()
	if prefix == "" {
		return id
	}
	return prefix + "-" + id
}
