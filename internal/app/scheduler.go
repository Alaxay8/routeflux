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

	lastHealthLoopConfigErr string
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
	go s.runRefreshLoop(ctx)
	go s.runHealthLoop(ctx)
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

// RunHealthOnce performs a single health-monitoring pass for auto mode.
func (s *Scheduler) RunHealthOnce(ctx context.Context) {
	s.runHealthOnce(ctx)
}

func (s *Scheduler) runRefreshLoop(ctx context.Context) {
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
}

func (s *Scheduler) runHealthLoop(ctx context.Context) {
	for {
		interval, enabled := s.healthLoopConfig()
		if !s.wait(ctx, interval) {
			return
		}
		if !enabled {
			continue
		}
		s.runHealthOnce(ctx)
	}
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

func (s *Scheduler) runHealthOnce(ctx context.Context) {
	if err := s.service.RunAutoHealthCheck(ctx); err != nil {
		s.logWarn("auto health check", "error", err.Error())
	}
}

func (s *Scheduler) healthLoopConfig() (time.Duration, bool) {
	if s.service == nil || s.service.store == nil {
		return time.Minute, false
	}

	settings, err := s.service.store.LoadSettings()
	if err != nil {
		s.logHealthLoopConfigError(err)
		return time.Minute, false
	}
	s.lastHealthLoopConfigErr = ""

	interval := settings.HealthCheckInterval.Duration()
	if interval > 0 {
		return interval, true
	}
	if s.tick > 0 {
		return s.tick, false
	}
	return time.Minute, false
}

func (s *Scheduler) wait(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		d = time.Minute
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-s.stopCh:
		return false
	case <-timer.C:
		return true
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

func (s *Scheduler) logHealthLoopConfigError(err error) {
	if err == nil {
		return
	}

	message := err.Error()
	if s.lastHealthLoopConfigErr == message {
		return
	}

	s.lastHealthLoopConfigErr = message
	s.logWarn("load settings for auto health loop", "error", message)
}
