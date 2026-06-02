package store

import (
	"fmt"

	"github.com/google/uuid"
)

func newID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("generate uuidv7: %w", err)
	}
	return id.String(), nil
}

func isUUIDv7(raw string) bool {
	id, err := uuid.Parse(raw)
	return err == nil && id.Version() == 7
}
