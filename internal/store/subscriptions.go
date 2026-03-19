package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/Alaxay8/routeflux/internal/domain"
)

// FileStore persists RouteFlux state as JSON files.
type FileStore struct {
	paths Paths
}

// NewFileStore creates a file-backed store rooted at the provided directory.
func NewFileStore(root string) *FileStore {
	return &FileStore{paths: NewPaths(root)}
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
