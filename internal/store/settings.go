package store

import (
	"errors"
	"fmt"
	"os"

	"github.com/Alaxay8/routeflux/internal/domain"
)

// SaveSettings persists user settings.
func (s *FileStore) SaveSettings(settings domain.Settings) error {
	settings.SchemaVersion = domain.DefaultSettings().SchemaVersion
	return AtomicWriteJSON(s.paths.SettingsPath, settings)
}

// LoadSettings loads persisted settings or returns defaults.
func (s *FileStore) LoadSettings() (domain.Settings, error) {
	defaults := domain.DefaultSettings()
	data, err := os.ReadFile(s.paths.SettingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return defaults, nil
		}
		return domain.Settings{}, err
	}

	settings, err := decodeSettings(data, s.paths.SettingsPath)
	if err == nil {
		return settings, nil
	}
	if errors.Is(err, ErrUnsupportedSettingsSchema) {
		return domain.Settings{}, err
	}
	if _, recoverErr := s.recoverCorruptJSON(s.paths.SettingsPath, defaults, err); recoverErr != nil {
		return domain.Settings{}, fmt.Errorf("recover settings: %w", recoverErr)
	}

	return defaults, nil
}
