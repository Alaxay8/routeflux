package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Alaxay8/routeflux/internal/backend"
	"github.com/Alaxay8/routeflux/internal/domain"
)

// ProxyUpdate changes only the supplied proxy settings in one transaction.
type ProxyUpdate struct {
	AllowLAN  *bool
	SOCKSPort *int
	HTTPPort  *int
}

// ConfigureProxy persists and applies explicit proxy listeners without changing routing or selection.
func (s *Service) ConfigureProxy(ctx context.Context, update ProxyUpdate) (domain.ProxySettings, error) {
	return runStoreWriteLockedResult(s, func() (domain.ProxySettings, error) {
		previous, err := s.store.LoadSettings()
		if err != nil {
			return domain.ProxySettings{}, fmt.Errorf("load settings: %w", err)
		}
		next := previous
		if update.AllowLAN != nil {
			next.Proxy.AllowLAN = *update.AllowLAN
		}
		if update.SOCKSPort != nil {
			next.Proxy.SOCKSPort = *update.SOCKSPort
		}
		if update.HTTPPort != nil {
			next.Proxy.HTTPPort = *update.HTTPPort
		}
		dnsPort, transparentPort := 0, 0
		if s.dns != nil && localDNSRuntimeEnabled(next.DNS) {
			dnsPort = localDNSPort
		}
		if firewallEnabled(next.Firewall) {
			transparentPort = next.Firewall.TransparentPort
		}
		if err := next.Proxy.Validate(dnsPort, transparentPort); err != nil {
			return domain.ProxySettings{}, err
		}
		if next.Proxy == previous.Proxy {
			return next.Proxy, nil
		}
		state, err := s.store.LoadState()
		if err != nil {
			return domain.ProxySettings{}, fmt.Errorf("load state: %w", err)
		}
		active := state.Connected && effectiveActiveTransport(state) == domain.TransportModeProxy && !state.ZapretTest.Active
		var request backend.ConfigRequest
		var snapshot backend.RollbackSnapshot
		if active {
			if s.backend == nil {
				return domain.ProxySettings{}, fmt.Errorf("backend is not configured")
			}
			_, node, err := s.subscriptionNode(state.ActiveSubscriptionID, state.ActiveNodeID)
			if err != nil {
				return domain.ProxySettings{}, err
			}
			runtimeSettings, err := s.prepareRuntimeDNSSettings(ctx, next)
			if err != nil {
				return domain.ProxySettings{}, fmt.Errorf("prepare DNS: %w", err)
			}
			resolved, err := s.resolveNodeAddress(ctx, node)
			if err != nil {
				return domain.ProxySettings{}, err
			}
			request = s.backendConfigRequest(runtimeSettings, resolved, state.Mode, 0, 0, firewallEnabled(next.Firewall), s.dns != nil && localDNSRuntimeEnabled(next.DNS))
			snapshot, err = s.backend.CaptureRollback()
			if err != nil {
				return domain.ProxySettings{}, fmt.Errorf("capture proxy rollback: %w", err)
			}
			if !snapshot.Available {
				return domain.ProxySettings{}, fmt.Errorf("cannot update active proxy without a rollback config")
			}
		}
		if err := s.store.SaveSettings(next); err != nil {
			return domain.ProxySettings{}, fmt.Errorf("save proxy settings: %w", err)
		}
		if !active {
			return next.Proxy, nil
		}
		applyErr := s.backend.ApplyConfig(ctx, request)
		if applyErr == nil {
			_, applyErr = s.ensureBackendRunning(ctx, state.ActiveSubscriptionID, state.ActiveNodeID, state.Mode)
		}
		if applyErr == nil {
			return next.Proxy, nil
		}
		// Rollback must still run when the caller's request was canceled.
		rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		settingsErr := s.store.SaveSettings(previous)
		runtimeErr := s.backend.RollbackConfig(rollbackCtx, snapshot)
		return domain.ProxySettings{}, errors.Join(fmt.Errorf("apply proxy settings: %w", applyErr), wrapProxyRollback("settings", settingsErr), wrapProxyRollback("runtime", runtimeErr))
	})
}

func wrapProxyRollback(part string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("restore proxy %s: %w", part, err)
}
