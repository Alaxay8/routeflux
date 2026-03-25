package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/Alaxay8/routeflux/internal/domain"
)

// FileStore persists RouteFlux state as JSON files.
type FileStore struct {
	paths  Paths
	logger *slog.Logger
}

// NewFileStore creates a file-backed store rooted at the provided directory.
func NewFileStore(root string) *FileStore {
	return &FileStore{paths: NewPaths(root)}
}

// WithLogger configures an optional logger for recovery warnings.
func (s *FileStore) WithLogger(logger *slog.Logger) *FileStore {
	s.logger = logger
	return s
}

// SaveSubscriptions persists all subscriptions.
func (s *FileStore) SaveSubscriptions(subscriptions []domain.Subscription) error {
	return AtomicWriteJSON(s.paths.SubscriptionsPath, subscriptions)
}

// LoadSubscriptions loads subscriptions, returning an empty list if the file does not exist.
func (s *FileStore) LoadSubscriptions() ([]domain.Subscription, error) {
	var subscriptions []domain.Subscription
	if err := readJSONFile(s.paths.SubscriptionsPath, &subscriptions); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []domain.Subscription{}, nil
		}
		return nil, err
	}

	if subscriptions == nil {
		return []domain.Subscription{}, nil
	}

	return subscriptions, nil
}

func readJSONFile(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("unmarshal %s: %w", path, err)
	}

	return nil
}

func (s *FileStore) logWarn(msg string, args ...any) {
	if s != nil && s.logger != nil {
		s.logger.Warn(msg, args...)
	}
}
