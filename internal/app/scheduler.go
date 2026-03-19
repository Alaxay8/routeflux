package app

import (
	"context"
	"time"
)

// Scheduler periodically refreshes subscriptions based on their configured interval.
type Scheduler struct {
	service *Service
	now     func() time.Time
	tick    time.Duration
	stopCh  chan struct{}
}

// NewScheduler creates a scheduler instance.
func NewScheduler(service *Service) *Scheduler {
	return &Scheduler{
		service: service,
		now:     time.Now,
		tick:    time.Minute,
		stopCh:  make(chan struct{}),
	}
}

// Start begins the background refresh loop.
func (s *Scheduler) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(s.tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.stopCh:
				return
			case <-ticker.C:
				s.runOnce(ctx)
			}
		}
	}()
}

// Stop terminates the background loop.
func (s *Scheduler) Stop() {
	select {
	case <-s.stopCh:
	default:
		close(s.stopCh)
	}
}

func (s *Scheduler) runOnce(ctx context.Context) {
	subscriptions, err := s.service.ListSubscriptions()
	if err != nil {
		return
	}

	for _, sub := range subscriptions {
		interval := sub.RefreshInterval.Duration()
		if interval <= 0 {
			continue
		}
		if s.now().UTC().Sub(sub.LastUpdatedAt) < interval {
			continue
		}
		_, _ = s.service.RefreshSubscription(ctx, sub.ID)
	}
}
