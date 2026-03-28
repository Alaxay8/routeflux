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

func TestFirewallHostCommandSupportsAllAlias(t *testing.T) {
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
	cmd.SetArgs([]string{"host", "*"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute firewall host all: %v", err)
	}

	if got := stdout.String(); !strings.Contains(got, "Host routing enabled for all") {
		t.Fatalf("unexpected output: %q", got)
	}

	settings, err := service.GetFirewallSettings()
	if err != nil {
		t.Fatalf("get firewall settings: %v", err)
	}
	if len(settings.SourceCIDRs) != 1 || settings.SourceCIDRs[0] != "all" {
		t.Fatalf("unexpected source hosts: %v", settings.SourceCIDRs)
	}
}

func TestFirewallSetHostsCommandSupportsAllAlias(t *testing.T) {
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
	cmd.SetArgs([]string{"set", "hosts", "all"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute firewall set hosts all: %v", err)
	}

	if got := stdout.String(); !strings.Contains(got, "Firewall hosts set to all") {
		t.Fatalf("unexpected output: %q", got)
	}

	settings, err := service.GetFirewallSettings()
	if err != nil {
		t.Fatalf("get firewall settings: %v", err)
	}
	if len(settings.SourceCIDRs) != 1 || settings.SourceCIDRs[0] != "all" {
		t.Fatalf("unexpected source hosts: %v", settings.SourceCIDRs)
	}
}

func TestFirewallGetShowsCurrentValuesAndMeaning(t *testing.T) {
	t.Parallel()

	store := &cliMemoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}
	store.settings.Firewall.Enabled = true
	store.settings.Firewall.SourceCIDRs = []string{"192.168.1.150"}
	store.settings.Firewall.BlockQUIC = true

	cmd := newFirewallCmd(&rootOptions{service: app.NewService(app.Dependencies{Store: store})})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"get"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute firewall get: %v", err)
	}

	output := stdout.String()
	wants := []string{
		"enabled=true",
		"mode=hosts",
		"mode-help=All TCP traffic from selected LAN devices goes through RouteFlux.",
		"hosts=192.168.1.150",
		"block-quic=true",
	}
	for _, want := range wants {
		if !strings.Contains(output, want) {
			t.Fatalf("firewall get missing %q\n%s", want, output)
		}
	}
}

func TestFirewallExplainOutputsFriendlyGuide(t *testing.T) {
	t.Parallel()

	cmd := newFirewallCmd(&rootOptions{service: app.NewService(app.Dependencies{Store: &cliMemoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}})})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"explain"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute firewall explain: %v", err)
	}

	output := stdout.String()
	wants := []string{
		"disabled: Do not redirect router traffic through RouteFlux.",
		"targets: Send traffic through RouteFlux only when the destination matches selected services, domains, or IPv4 targets.",
		"Service presets: youtube, instagram, discord, whatsapp, telegram-web, telegram, facetime.",
		"Popular root domains like youtube.com and instagram.com still auto-expand to the domain families they need.",
		"hosts: Send all TCP traffic from selected LAN devices through RouteFlux.",
		"all or *: all common private LAN ranges",
		"routeflux firewall set hosts 192.168.1.150",
	}
	for _, want := range wants {
		if !strings.Contains(output, want) {
			t.Fatalf("firewall explain missing %q\n%s", want, output)
		}
	}
}

func TestSettingsGetIncludesFirewallHosts(t *testing.T) {
	t.Parallel()

	store := &cliMemoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}
	store.settings.Firewall.Enabled = true
	store.settings.Firewall.TargetServices = []string{"youtube"}
	store.settings.Firewall.TargetCIDRs = []string{"1.1.1.1"}
	store.settings.Firewall.TargetDomains = []string{"youtube.com"}
	store.settings.Firewall.SourceCIDRs = []string{"192.168.1.150"}
	store.settings.Firewall.BlockQUIC = true

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
		"firewall-targets=youtube, youtube.com, 1.1.1.1",
		"firewall-target-services=youtube",
		"firewall-target-domains=youtube.com",
		"firewall-target-cidrs=1.1.1.1",
		"firewall-hosts=192.168.1.150",
		"firewall-block-quic=true",
	}
	for _, want := range wants {
		if !strings.Contains(output, want) {
			t.Fatalf("settings output missing %q\n%s", want, output)
		}
	}
}

func TestFirewallSetTargetsSupportsServicesAndDomains(t *testing.T) {
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
	cmd.SetArgs([]string{"set", "targets", "YouTube", "YouTube.com", "1.1.1.1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute firewall set targets: %v", err)
	}

	settings, err := service.GetFirewallSettings()
	if err != nil {
		t.Fatalf("get firewall settings: %v", err)
	}
	if len(settings.TargetServices) != 1 || settings.TargetServices[0] != "youtube" {
		t.Fatalf("unexpected target services: %v", settings.TargetServices)
	}
	if len(settings.TargetDomains) != 1 || settings.TargetDomains[0] != "youtube.com" {
		t.Fatalf("unexpected target domains: %v", settings.TargetDomains)
	}
	if len(settings.TargetCIDRs) != 1 || settings.TargetCIDRs[0] != "1.1.1.1" {
		t.Fatalf("unexpected target cidrs: %v", settings.TargetCIDRs)
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
