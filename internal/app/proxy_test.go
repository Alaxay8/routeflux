package app

import (
	"context"
	"errors"
	"github.com/Alaxay8/routeflux/internal/backend"
	"github.com/Alaxay8/routeflux/internal/domain"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestProxyUpdateDisconnectedAndValidation(t *testing.T) {
	st := &memoryStore{settings: domain.DefaultSettings(), state: domain.DefaultRuntimeState()}
	b := &recordingBackend{}
	s := NewService(Dependencies{Store: st, Backend: b})
	lan := true
	port := 20809
	got, err := s.ConfigureProxy(context.Background(), ProxyUpdate{AllowLAN: &lan, HTTPPort: &port})
	if err != nil {
		t.Fatal(err)
	}
	if !got.AllowLAN || got.HTTPPort != port || len(b.requests) != 0 || b.stopCalls != 0 {
		t.Fatalf("unexpected result: %+v", got)
	}
	for _, p := range []int{0, -1, 65536, 10808} {
		_, err = s.ConfigureProxy(context.Background(), ProxyUpdate{HTTPPort: &p})
		if err == nil {
			t.Fatalf("accepted port %d", p)
		}
		if st.settings.Proxy != got {
			t.Fatal("invalid update was saved")
		}
	}
	st.settings.Firewall.Enabled = true
	st.settings.Firewall.Mode = domain.FirewallModeHosts
	st.settings.Firewall.Hosts = []string{"192.168.1.2"}
	p := st.settings.Firewall.TransparentPort
	if _, err = s.ConfigureProxy(context.Background(), ProxyUpdate{HTTPPort: &p}); err == nil {
		t.Fatal("accepted transparent port collision")
	}
}

type proxyFailBackend struct {
	recordingBackend
	fail error
}

func (b *proxyFailBackend) ApplyConfig(ctx context.Context, r backend.ConfigRequest) error {
	b.requests = append(b.requests, r)
	return b.fail
}

func TestProxyUpdateRuntimeAndRollback(t *testing.T) {
	for _, fail := range []bool{false, true} {
		st := &memoryStore{settings: domain.DefaultSettings(), state: domain.DefaultRuntimeState(), subs: []domain.Subscription{{ID: "s", Nodes: []domain.Node{{ID: "n", Address: "127.0.0.1", Port: 9999, Protocol: domain.ProtocolSocks}}}}}
		st.settings.DNS.Mode = domain.DNSModeSystem
		st.state.Connected = true
		st.state.ActiveSubscriptionID = "s"
		st.state.ActiveNodeID = "n"
		st.state.ActiveTransport = domain.TransportModeProxy
		before := st.settings
		state := st.state
		b := &proxyFailBackend{}
		if fail {
			b.fail = errors.New("bind failed")
		}
		s := NewService(Dependencies{Store: st, Backend: b})
		lan := true
		p := 20809
		_, err := s.ConfigureProxy(context.Background(), ProxyUpdate{AllowLAN: &lan, HTTPPort: &p})
		if (err != nil) != fail {
			t.Fatalf("fail %t: %v", fail, err)
		}
		if !reflect.DeepEqual(st.state, state) {
			t.Fatal("proxy update changed selection state")
		}
		if fail {
			if !reflect.DeepEqual(st.settings, before) || b.rollbackCalls != 1 {
				t.Fatal("failed update did not roll back")
			}
		} else {
			if len(b.requests) != 1 || !b.requests[0].AllowLAN || b.requests[0].HTTPPort != p || b.requests[0].TransparentProxy {
				t.Fatalf("request: %+v", b.requests)
			}
		}
	}
}

func TestProxyEgressUsesConfiguredHTTPPort(t *testing.T) {
	requests := make(chan struct{}, 4)
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- struct{}{}
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer proxy.Close()
	st := &memoryStore{settings: domain.DefaultSettings()}
	st.settings.Proxy.HTTPPort = proxy.Listener.Addr().(*net.TCPAddr).Port
	s := NewService(Dependencies{Store: st})
	// The fake proxy rejects CONNECT: reaching it proves no hardcoded port or direct network fallback is used.
	if err := s.defaultBackendEgressProbe(context.Background()); err == nil {
		t.Fatal("expected CONNECT rejection")
	}
	select {
	case <-requests:
	default:
		t.Fatal("egress probe did not use configured HTTP proxy")
	}
}

func TestProxyRuntimeRequestAndIsolatedProbe(t *testing.T) {
	settings := domain.DefaultSettings()
	settings.Proxy = domain.ProxySettings{AllowLAN: true, SOCKSPort: 20808, HTTPPort: 20809}
	s := NewService(Dependencies{Store: &memoryStore{settings: settings}})
	for _, ports := range [][2]int{{0, 0}, {30001, 30002}} {
		req := s.backendConfigRequest(settings, domain.Node{ID: "n"}, domain.SelectionModeManual, ports[0], ports[1], false, false)
		if ports[0] == 0 {
			if !req.AllowLAN || req.SOCKSPort != 20808 || req.HTTPPort != 20809 {
				t.Fatalf("runtime: %+v", req)
			}
		} else {
			if req.AllowLAN || req.SOCKSPort != 30001 || req.HTTPPort != 30002 {
				t.Fatalf("probe: %+v", req)
			}
		}
	}
}

func TestProxySettingsSurviveConnectionWorkflows(t *testing.T) {
	st := &memoryStore{settings: domain.DefaultSettings(), state: domain.DefaultRuntimeState(), subs: []domain.Subscription{{ID: "server-list", Nodes: []domain.Node{
		{ID: "one", Name: "one", Address: "127.0.0.1", Port: 9001, Protocol: domain.ProtocolSocks},
		{ID: "two", Name: "two", Address: "127.0.0.1", Port: 9002, Protocol: domain.ProtocolSocks},
	}}}}
	st.settings.DNS.Mode = domain.DNSModeSystem
	want := domain.ProxySettings{AllowLAN: true, SOCKSPort: 20808, HTTPPort: 20809}
	st.settings.Proxy = want
	b := &recordingBackend{}
	s := NewService(Dependencies{Store: st, Backend: b})
	for _, node := range []string{"one", "two"} {
		if err := s.ConnectManual(context.Background(), "server-list", node); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.RefreshSubscription(context.Background(), "server-list"); err != nil {
		t.Fatal(err)
	}
	if err := s.RestoreRuntime(context.Background()); err != nil {
		t.Fatal(err)
	}
	if st.settings.Proxy != want {
		t.Fatal("proxy settings lost")
	}
	if len(b.requests) != 3 {
		t.Fatalf("applied %d requests", len(b.requests))
	}
	for _, req := range b.requests {
		if !req.AllowLAN || req.SOCKSPort != want.SOCKSPort || req.HTTPPort != want.HTTPPort {
			t.Fatalf("request lost settings: %+v", req)
		}
	}
}

func TestProxyUpdateRollsBackWhenStatusFails(t *testing.T) {
	st := &memoryStore{settings: domain.DefaultSettings(), state: domain.DefaultRuntimeState(), subs: []domain.Subscription{{ID: "s", Nodes: []domain.Node{{ID: "n", Address: "127.0.0.1", Port: 9001, Protocol: domain.ProtocolSocks}}}}}
	st.settings.DNS.Mode = domain.DNSModeSystem
	st.state.Connected = true
	st.state.ActiveTransport = domain.TransportModeProxy
	st.state.ActiveSubscriptionID = "s"
	st.state.ActiveNodeID = "n"
	b := &recordingBackend{statusErr: errors.New("runtime unavailable")}
	s := NewService(Dependencies{Store: st, Backend: b})
	lan := true
	if _, err := s.ConfigureProxy(context.Background(), ProxyUpdate{AllowLAN: &lan}); err == nil {
		t.Fatal("expected runtime error")
	}
	if st.settings.Proxy.AllowLAN || b.rollbackCalls != 1 {
		t.Fatal("runtime failure did not roll back")
	}
}
