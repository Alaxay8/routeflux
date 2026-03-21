package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Alaxay8/routeflux/internal/app"
	"github.com/Alaxay8/routeflux/internal/domain"
)

func TestFirewallHostCommand(t *testing.T) {
	t.Parallel()

	store := &cliMemoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}
	service := app.NewService(app.Dependencies{Store: store})

	cmd := newFirewallCmd(&rootOptions{service: service})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"host", "192.168.1.150"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute firewall host: %v", err)
	}

	if got := stdout.String(); !strings.Contains(got, "Host routing enabled for 192.168.1.150") {
		t.Fatalf("unexpected output: %q", got)
	}

	settings, err := service.GetFirewallSettings()
	if err != nil {
		t.Fatalf("get firewall settings: %v", err)
	}
	if len(settings.SourceCIDRs) != 1 || settings.SourceCIDRs[0] != "192.168.1.150" {
		t.Fatalf("unexpected source hosts: %v", settings.SourceCIDRs)
	}
}

func TestSettingsGetIncludesFirewallHosts(t *testing.T) {
	t.Parallel()

	store := &cliMemoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}
	store.settings.Firewall.Enabled = true
	store.settings.Firewall.TargetCIDRs = []string{"1.1.1.1"}
	store.settings.Firewall.SourceCIDRs = []string{"192.168.1.150"}
	store.settings.Firewall.BlockQUIC = true
	store.settings.DNS.Mode = domain.DNSModeSplit
	store.settings.DNS.Transport = domain.DNSTransportDoH
	store.settings.DNS.Servers = []string{"dns.google", "1.1.1.1"}
	store.settings.DNS.Bootstrap = []string{"9.9.9.9"}
	store.settings.DNS.DirectDomains = []string{"domain:lan", "full:router.lan"}

	cmd := newSettingsCmd(&rootOptions{service: app.NewService(app.Dependencies{Store: store})})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"get"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute settings get: %v", err)
	}

	output := stdout.String()
	wants := []string{
		"firewall-targets=1.1.1.1",
		"firewall-hosts=192.168.1.150",
		"firewall-block-quic=true",
		"dns.mode=split",
		"dns.transport=doh",
		"dns.servers=dns.google, 1.1.1.1",
		"dns.bootstrap=9.9.9.9",
		"dns.direct-domains=domain:lan, full:router.lan",
	}
	for _, want := range wants {
		if !strings.Contains(output, want) {
			t.Fatalf("settings output missing %q\n%s", want, output)
		}
	}
}

type cliMemoryStore struct {
	subs     []domain.Subscription
	settings domain.Settings
	state    domain.RuntimeState
}

func (s *cliMemoryStore) LoadSubscriptions() ([]domain.Subscription, error) {
	return s.subs, nil
}

func (s *cliMemoryStore) SaveSubscriptions(subs []domain.Subscription) error {
	s.subs = subs
	return nil
}

func (s *cliMemoryStore) LoadSettings() (domain.Settings, error) {
	return s.settings, nil
}

func (s *cliMemoryStore) SaveSettings(settings domain.Settings) error {
	s.settings = settings
	return nil
}

func (s *cliMemoryStore) LoadState() (domain.RuntimeState, error) {
	return s.state, nil
}

func (s *cliMemoryStore) SaveState(state domain.RuntimeState) error {
	s.state = state
	return nil
}
