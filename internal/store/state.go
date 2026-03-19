package store

import (
	"errors"
	"os"
	"time"

	"github.com/Alaxay8/routeflux/internal/domain"
)

// SaveState persists runtime state.
func (s *FileStore) SaveState(state domain.RuntimeState) error {
	return AtomicWriteJSON(s.paths.StatePath, state)
}

// LoadState loads persisted runtime state or returns the default state.
func (s *FileStore) LoadState() (domain.RuntimeState, error) {
	state := domain.DefaultRuntimeState()
	if err := readJSONFile(s.paths.StatePath, &state); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return domain.DefaultRuntimeState(), nil
		}
		return domain.RuntimeState{}, err
	}

	if state.LastRefreshAt == nil {
		state.LastRefreshAt = make(map[string]time.Time)
	}
	if state.Health == nil {
		state.Health = make(map[string]domain.NodeHealth)
	}

	return state, nil
}
