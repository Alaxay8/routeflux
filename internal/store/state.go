package store

import (
	"errors"
	"fmt"
	"os"

	"github.com/Alaxay8/routeflux/internal/domain"
)

// SaveState persists runtime state.
func (s *FileStore) SaveState(state domain.RuntimeState) error {
	state.SchemaVersion = domain.DefaultRuntimeState().SchemaVersion
	return AtomicWriteJSON(s.paths.StatePath, state)
}

// LoadState loads persisted runtime state or returns the default state.
func (s *FileStore) LoadState() (domain.RuntimeState, error) {
	defaults := domain.DefaultRuntimeState()
	data, err := os.ReadFile(s.paths.StatePath)
	if err != nil {
		if os.IsNotExist(err) {
			return defaults, nil
		}
		return domain.RuntimeState{}, err
	}

	state, err := decodeState(data, s.paths.StatePath)
	if err == nil {
		return state, nil
	}
	if errors.Is(err, ErrUnsupportedStateSchema) {
		return domain.RuntimeState{}, err
	}
	if _, recoverErr := s.recoverCorruptJSON(s.paths.StatePath, defaults, err); recoverErr != nil {
		return domain.RuntimeState{}, fmt.Errorf("recover state: %w", recoverErr)
	}

	return defaults, nil
}
