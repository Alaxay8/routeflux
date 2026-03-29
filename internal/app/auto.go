package app

import (
	"context"
	"fmt"
	"time"

	"github.com/Alaxay8/routeflux/internal/domain"
	"github.com/Alaxay8/routeflux/internal/probe"
)

type autoSelectionDecision struct {
	CurrentNodeID       string
	CandidateNode       domain.Node
	CandidateScore      domain.ScoreResult
	SelectedNode        domain.Node
	Health              map[string]domain.NodeHealth
	HasHealthyCandidate bool
	Switch              bool
	Reconnect           bool
	Reason              string
}

// RunAutoHealthCheck probes the active auto-mode subscription and reconnects when needed.
func (s *Service) RunAutoHealthCheck(ctx context.Context) error {
	return runStoreWriteLocked(s, func() error {
		return s.runAutoHealthCheck(ctx)
	})
}

func (s *Service) runAutoHealthCheck(ctx context.Context) error {
	settings, err := s.store.LoadSettings()
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}

	persistedState, err := s.store.LoadState()
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}
	state := s.mergeAutoHealthState(persistedState)
	if state.Mode != domain.SelectionModeAuto || state.ActiveSubscriptionID == "" {
		return nil
	}

	sub, err := s.subscriptionByID(state.ActiveSubscriptionID)
	if err != nil {
		return err
	}

	decision, err := s.evaluateAutoSelection(ctx, sub, settings, state)
	if err != nil {
		return err
	}

	s.logAutoDecision("auto health decision", sub, decision)

	if !decision.HasHealthyCandidate {
		return s.persistAutoFailure(ctx, sub, state, decision)
	}

	if !decision.Reconnect && !decision.Switch {
		state.Health = decision.Health
		state.LastFailureReason = ""
		if s.shouldPersistAutoHealthState(persistedState, state) {
			if err := s.saveState(state); err != nil {
				return fmt.Errorf("save state: %w", err)
			}
		} else {
			s.rememberAutoHealthState(state, false)
		}
		return nil
	}

	_, err = s.commitAutoSelection(ctx, sub, decision)
	return err
}

func (s *Service) evaluateAutoSelection(ctx context.Context, sub domain.Subscription, settings domain.Settings, state domain.RuntimeState) (autoSelectionDecision, error) {
	health := cloneHealthMap(state.Health)
	if health == nil {
		health = make(map[string]domain.NodeHealth)
	}

	currentNodeID := ""
	if state.ActiveSubscriptionID == sub.ID {
		currentNodeID = state.ActiveNodeID
	}

	s.probeSubscription(ctx, sub, health)

	candidateNode, candidateScore, err := probe.SelectBestNode(sub.Nodes, health, probe.DefaultScoreConfig())
	if err != nil {
		return autoSelectionDecision{}, err
	}

	candidateHealth := health[candidateNode.ID]
	decision := autoSelectionDecision{
		CurrentNodeID:       currentNodeID,
		CandidateNode:       candidateNode,
		CandidateScore:      candidateScore,
		Health:              health,
		HasHealthyCandidate: candidateHealth.Healthy,
		Reason:              "no healthy nodes available",
	}
	if !candidateHealth.Healthy {
		return decision, nil
	}

	if !state.Connected {
		decision.SelectedNode = candidateNode
		decision.Switch = candidateNode.ID != currentNodeID
		decision.Reconnect = true
		decision.Reason = "recover disconnected auto mode"
		return decision, nil
	}

	currentHealth := health[currentNodeID]
	shouldSwitch, reason := probe.ShouldSwitch(currentHealth, candidateHealth, time.Now().UTC(), state.LastSwitchAt, switchPolicyFromSettings(settings))

	selectedNode := candidateNode
	if !shouldSwitch && currentNodeID != "" {
		activeNode, ok := sub.NodeByID(currentNodeID)
		if ok {
			selectedNode = activeNode
		} else {
			shouldSwitch = true
			reason = "current node missing"
		}
	}

	if shouldSwitch && reason == "" {
		reason = "candidate selected"
	}

	decision.SelectedNode = selectedNode
	decision.Switch = selectedNode.ID != currentNodeID
	decision.Reason = reason
	return decision, nil
}

func (s *Service) commitAutoSelection(ctx context.Context, sub domain.Subscription, decision autoSelectionDecision) (domain.Node, error) {
	if err := s.applyNodeSelection(ctx, sub, decision.SelectedNode, domain.SelectionModeAuto); err != nil {
		return domain.Node{}, err
	}

	state, err := s.store.LoadState()
	if err != nil {
		return domain.Node{}, fmt.Errorf("load state: %w", err)
	}

	state.Health = decision.Health
	state.Mode = domain.SelectionModeAuto
	state.Connected = true
	state.ActiveSubscriptionID = sub.ID
	state.ActiveNodeID = decision.SelectedNode.ID
	if decision.Switch {
		state.LastSwitchAt = time.Now().UTC()
	}

	if err := s.saveState(state); err != nil {
		return domain.Node{}, fmt.Errorf("save state: %w", err)
	}

	return decision.SelectedNode, nil
}

func (s *Service) persistAutoFailure(ctx context.Context, sub domain.Subscription, state domain.RuntimeState, decision autoSelectionDecision) error {
	if state.Connected {
		failedNodeID := decision.CurrentNodeID
		if failedNodeID == "" {
			failedNodeID = state.ActiveNodeID
		}
		if err := s.markConnectionFailed(ctx, sub.ID, failedNodeID, domain.SelectionModeAuto, decision.Reason); err != nil {
			return err
		}

		reloaded, err := s.store.LoadState()
		if err != nil {
			return fmt.Errorf("load state: %w", err)
		}
		state = reloaded
	}

	state.Health = decision.Health
	state.Mode = domain.SelectionModeAuto
	state.Connected = false
	if state.ActiveSubscriptionID == "" {
		state.ActiveSubscriptionID = sub.ID
	}
	if state.ActiveNodeID == "" {
		state.ActiveNodeID = decision.CurrentNodeID
	}
	state.LastFailureReason = decision.Reason

	if err := s.saveState(state); err != nil {
		return fmt.Errorf("save state: %w", err)
	}

	return nil
}

func (s *Service) logAutoDecision(msg string, sub domain.Subscription, decision autoSelectionDecision) {
	selectedNodeID := decision.SelectedNode.ID
	if !decision.HasHealthyCandidate {
		selectedNodeID = ""
	}

	s.logInfo(
		msg,
		"subscription", sub.ID,
		"current_node", decision.CurrentNodeID,
		"candidate_node", decision.CandidateNode.ID,
		"selected_node", selectedNodeID,
		"switch", decision.Switch,
		"reconnect", decision.Reconnect,
		"reason", decision.Reason,
		"candidate_score", decision.CandidateScore.Score,
	)
}

func switchPolicyFromSettings(settings domain.Settings) probe.SwitchPolicy {
	policy := probe.DefaultSwitchPolicy()
	if settings.SwitchCooldown.Duration() >= 0 {
		policy.Cooldown = settings.SwitchCooldown.Duration()
	}
	if settings.LatencyThreshold.Duration() >= 0 {
		policy.LatencyImprovement = settings.LatencyThreshold.Duration()
	}
	return policy
}

func cloneHealthMap(source map[string]domain.NodeHealth) map[string]domain.NodeHealth {
	cloned := make(map[string]domain.NodeHealth, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}
