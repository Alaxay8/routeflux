package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// WithWriteLock runs fn while holding the store's inter-process write lock.
func (s *FileStore) WithWriteLock(fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(s.paths.LockPath), PrivateDirPerm); err != nil {
		return fmt.Errorf("create lock directory: %w", err)
	}

	file, err := os.OpenFile(s.paths.LockPath, os.O_CREATE|os.O_RDWR, SecretFilePerm)
	if err != nil {
		return fmt.Errorf("open lock file: %w", err)
	}
	defer file.Close()

	if err := fcntlLock(file, syscall.F_WRLCK); err != nil {
		return fmt.Errorf("acquire write lock: %w", err)
	}
	defer func() {
		_ = fcntlLock(file, syscall.F_UNLCK)
	}()

	return fn()
}

func fcntlLock(file *os.File, lockType int16) error {
	lock := syscall.Flock_t{
		Type:   lockType,
		Whence: 0,
		Start:  0,
		Len:    0,
	}

	for {
		if err := syscall.FcntlFlock(file.Fd(), syscall.F_SETLKW, &lock); err != nil {
			if errors.Is(err, syscall.EINTR) {
				continue
			}
			return err
		}
		return nil
	}
}
