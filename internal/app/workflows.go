package app

import "context"

// RefreshAndReconnect refreshes the current subscription and reapplies the active mode.
func (s *Service) RefreshAndReconnect(ctx context.Context) error {
	status, err := s.Status()
	if err != nil {
		return err
	}

	if status.State.ActiveSubscriptionID == "" {
		return nil
	}

	sub, err := s.RefreshSubscription(ctx, status.State.ActiveSubscriptionID)
	if err != nil {
		return err
	}

	switch status.State.Mode {
	case "manual":
		if status.State.ActiveNodeID == "" {
			return nil
		}
		return s.ConnectManual(ctx, sub.ID, status.State.ActiveNodeID)
	case "auto":
		_, err := s.ConnectAuto(ctx, sub.ID)
		return err
	default:
		return nil
	}
}
