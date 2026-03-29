package app

import (
	"time"

	"github.com/Alaxay8/routeflux/internal/domain"
)

const autoHealthStatePersistInterval = 5 * time.Minute

type autoHealthStateKey struct {
	ActiveSubscriptionID string
	ActiveNodeID         string
	Mode                 domain.SelectionMode
	Connected            bool
	LastSwitchAt         time.Time
}

type autoHealthStateCache struct {
	key               autoHealthStateKey
	health            map[string]domain.NodeHealth
	lastFailureReason string
	lastPersistAt     time.Time
}

func (s *Service) loadStateWithAutoHealthCache() (domain.RuntimeState, error) {
	state, err := s.store.LoadState()
	if err != nil {
		return domain.RuntimeState{}, err
	}

	return s.mergeAutoHealthState(state), nil
}

func (s *Service) saveState(state domain.RuntimeState) error {
	if err := s.store.SaveState(state); err != nil {
		return err
	}
	s.rememberAutoHealthState(state, true)
	return nil
}

func (s *Service) mergeAutoHealthState(state domain.RuntimeState) domain.RuntimeState {
	if s == nil {
		return state
	}

	s.autoHealthStateMu.Lock()
	defer s.autoHealthStateMu.Unlock()

	if s.autoHealthState == nil || !s.autoHealthState.key.matches(state) {
		return state
	}

	state.Health = cloneHealthMap(s.autoHealthState.health)
	state.LastFailureReason = s.autoHealthState.lastFailureReason
	return state
}

func (s *Service) rememberAutoHealthState(state domain.RuntimeState, persisted bool) {
	if s == nil {
		return
	}

	s.autoHealthStateMu.Lock()
	defer s.autoHealthStateMu.Unlock()

	if state.Mode != domain.SelectionModeAuto || state.ActiveSubscriptionID == "" {
		s.autoHealthState = nil
		return
	}

	lastPersistAt := time.Time{}
	if s.autoHealthState != nil {
		lastPersistAt = s.autoHealthState.lastPersistAt
	}
	if persisted {
		lastPersistAt = s.currentTime().UTC()
	}

	s.autoHealthState = &autoHealthStateCache{
		key:               autoHealthStateKeyFromState(state),
		health:            cloneHealthMap(state.Health),
		lastFailureReason: state.LastFailureReason,
		lastPersistAt:     lastPersistAt,
	}
}

func (s *Service) shouldPersistAutoHealthState(persistedState, nextState domain.RuntimeState) bool {
	if s == nil {
		return true
	}

	s.autoHealthStateMu.Lock()
	defer s.autoHealthStateMu.Unlock()

	if s.autoHealthState == nil || !s.autoHealthState.key.matches(persistedState) {
		return true
	}
	if persistedState.LastFailureReason != nextState.LastFailureReason {
		return true
	}
	if s.autoHealthState.lastPersistAt.IsZero() {
		return true
	}

	return s.currentTime().UTC().Sub(s.autoHealthState.lastPersistAt) >= autoHealthStatePersistInterval
}

func (s *Service) currentTime() time.Time {
	if s != nil && s.now != nil {
		return s.now()
	}
	return time.Now()
}

func autoHealthStateKeyFromState(state domain.RuntimeState) autoHealthStateKey {
	return autoHealthStateKey{
		ActiveSubscriptionID: state.ActiveSubscriptionID,
		ActiveNodeID:         state.ActiveNodeID,
		Mode:                 state.Mode,
		Connected:            state.Connected,
		LastSwitchAt:         state.LastSwitchAt,
	}
}

func (k autoHealthStateKey) matches(state domain.RuntimeState) bool {
	return k == autoHealthStateKeyFromState(state)
}
