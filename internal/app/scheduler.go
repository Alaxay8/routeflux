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

// SetTick overrides the scheduler tick interval.
func (s *Scheduler) SetTick(tick time.Duration) {
	if tick > 0 {
		s.tick = tick
	}
}

// Start begins the background refresh loop.
func (s *Scheduler) Start(ctx context.Context) {
	go func() {
		s.RunOnce(ctx)

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

// RunOnce performs a single refresh scan across stored subscriptions.
func (s *Scheduler) RunOnce(ctx context.Context) {
	s.runOnce(ctx)
}

func (s *Scheduler) runOnce(ctx context.Context) {
	subscriptions, err := s.service.ListSubscriptions()
	if err != nil {
		s.logWarn("list subscriptions for scheduler", "error", err.Error())
		return
	}

	status, statusErr := s.service.Status()
	activeSubscriptionID := ""
	connected := false
	if statusErr == nil {
		activeSubscriptionID = status.State.ActiveSubscriptionID
		connected = status.State.Connected
	}

	for _, sub := range subscriptions {
		interval := sub.RefreshInterval.Duration()
		if interval <= 0 {
			continue
		}
		if s.now().UTC().Sub(sub.LastUpdatedAt) < interval {
			continue
		}

		if connected && sub.ID == activeSubscriptionID {
			if err := s.service.RefreshAndReconnect(ctx); err != nil {
				s.logWarn("refresh and reconnect active subscription", "subscription", sub.ID, "error", err.Error())
				continue
			}
			s.logInfo("refreshed and reconnected active subscription", "subscription", sub.ID)
			continue
		}

		if _, err := s.service.RefreshSubscription(ctx, sub.ID); err != nil {
			s.logWarn("refresh subscription", "subscription", sub.ID, "error", err.Error())
			continue
		}
		s.logInfo("refreshed subscription", "subscription", sub.ID)
	}
}

func (s *Scheduler) logInfo(msg string, args ...any) {
	if s.service != nil && s.service.logger != nil {
		s.service.logger.Info(msg, args...)
	}
}

func (s *Scheduler) logWarn(msg string, args ...any) {
	if s.service != nil && s.service.logger != nil {
		s.service.logger.Warn(msg, args...)
	}
}
