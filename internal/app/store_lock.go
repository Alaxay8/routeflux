package app

type storeWriteLocker interface {
	WithWriteLock(func() error) error
}

func runStoreWriteLocked(s *Service, fn func() error) error {
	locker, ok := s.store.(storeWriteLocker)
	if !ok {
		return fn()
	}

	return locker.WithWriteLock(fn)
}

func runStoreWriteLockedResult[T any](s *Service, fn func() (T, error)) (T, error) {
	locker, ok := s.store.(storeWriteLocker)
	if !ok {
		return fn()
	}

	var result T
	err := locker.WithWriteLock(func() error {
		var innerErr error
		result, innerErr = fn()
		return innerErr
	})
	return result, err
}
