package store

import "path/filepath"

// Paths defines the persisted file layout.
type Paths struct {
	Root              string
	SubscriptionsPath string
	SettingsPath      string
	StatePath         string
}

// NewPaths constructs the default file layout for a root directory.
func NewPaths(root string) Paths {
	return Paths{
		Root:              root,
		SubscriptionsPath: filepath.Join(root, "subscriptions.json"),
		SettingsPath:      filepath.Join(root, "settings.json"),
		StatePath:         filepath.Join(root, "state.json"),
	}
}
