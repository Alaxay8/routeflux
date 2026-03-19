package store

import (
	"errors"
	"os"

	"github.com/Alaxay8/routeflux/internal/domain"
)

// SaveSettings persists user settings.
func (s *FileStore) SaveSettings(settings domain.Settings) error {
	return AtomicWriteJSON(s.paths.SettingsPath, settings)
}

// LoadSettings loads persisted settings or returns defaults.
func (s *FileStore) LoadSettings() (domain.Settings, error) {
	settings := domain.DefaultSettings()
	if err := readJSONFile(s.paths.SettingsPath, &settings); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return domain.DefaultSettings(), nil
		}
		return domain.Settings{}, err
	}

	return settings, nil
}
