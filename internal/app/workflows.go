package app

import "context"

// RefreshAndReconnect refreshes the current subscription and reapplies the active mode.
func (s *Service) RefreshAndReconnect(ctx context.Context) error {
	return runStoreWriteLocked(s, func() error {
		return s.refreshAndReconnect(ctx)
	})
}

func (s *Service) refreshAndReconnect(ctx context.Context) error {
	status, err := s.Status()
	if err != nil {
		return err
	}

	if status.State.ActiveSubscriptionID == "" {
		return nil
	}

	sub, err := s.refreshSubscription(ctx, status.State.ActiveSubscriptionID)
	if err != nil {
		return err
	}

	switch status.State.Mode {
	case "manual":
		if status.State.ActiveNodeID == "" {
			return nil
		}
		return s.connectManual(ctx, sub.ID, status.State.ActiveNodeID)
	case "auto":
		_, err := s.connectAuto(ctx, sub.ID)
		return err
	default:
		return nil
	}
}
