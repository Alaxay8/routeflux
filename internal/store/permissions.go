package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	// SecretFilePerm restricts secret-bearing files to the owner.
	SecretFilePerm os.FileMode = 0o600
	// PrivateDirPerm restricts RouteFlux-owned state directories to the owner.
	PrivateDirPerm os.FileMode = 0o700
)

// HardenSecretPermissions best-effort hardens RouteFlux and Xray secret-bearing files.
func (s *FileStore) HardenSecretPermissions(xrayConfigPath string) error {
	var errs []error

	if err := chmodDirIfExists(s.paths.Root, PrivateDirPerm); err != nil {
		errs = append(errs, fmt.Errorf("chmod routeflux root: %w", err))
	}

	for _, path := range s.secretFilePaths(xrayConfigPath) {
		if err := chmodFileIfExists(path, SecretFilePerm); err != nil {
			errs = append(errs, fmt.Errorf("chmod %s: %w", path, err))
		}
	}

	corruptBackups, err := filepath.Glob(filepath.Join(s.paths.Root, "*.corrupt-*"))
	if err != nil {
		errs = append(errs, fmt.Errorf("list corrupt backups: %w", err))
	} else {
		for _, path := range corruptBackups {
			if err := chmodFileIfExists(path, SecretFilePerm); err != nil {
				errs = append(errs, fmt.Errorf("chmod %s: %w", path, err))
			}
		}
	}

	return errors.Join(errs...)
}

func (s *FileStore) secretFilePaths(xrayConfigPath string) []string {
	paths := []string{
		s.paths.SubscriptionsPath,
		s.paths.SettingsPath,
		s.paths.StatePath,
		s.paths.LockPath,
		filepath.Join(s.paths.Root, "speedtest.lock"),
	}

	if xrayConfigPath != "" {
		paths = append(paths, xrayConfigPath, xrayConfigPath+".last-known-good")
	}

	return paths
}

func chmodFileIfExists(path string, perm os.FileMode) error {
	if path == "" {
		return nil
	}

	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("expected file, got directory")
	}

	if err := os.Chmod(path, perm); err != nil {
		return err
	}

	return nil
}

func chmodDirIfExists(path string, perm os.FileMode) error {
	if path == "" {
		return nil
	}

	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("expected directory, got file")
	}

	if err := os.Chmod(path, perm); err != nil {
		return err
	}

	return nil
}
