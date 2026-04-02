package cli

import (
	"bytes"
	"reflect"
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
	if len(settings.Hosts) != 1 || settings.Hosts[0] != "192.168.1.150" {
		t.Fatalf("unexpected source hosts: %v", settings.Hosts)
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
	if len(settings.Hosts) != 1 || settings.Hosts[0] != "all" {
		t.Fatalf("unexpected source hosts: %v", settings.Hosts)
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
	if len(settings.Hosts) != 1 || settings.Hosts[0] != "all" {
		t.Fatalf("unexpected source hosts: %v", settings.Hosts)
	}
}

func TestFirewallGetShowsCurrentValuesAndMeaning(t *testing.T) {
	t.Parallel()

	store := &cliMemoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}
	store.settings.Firewall.Enabled = true
	store.settings.Firewall.Mode = domain.FirewallModeHosts
	store.settings.Firewall.Hosts = []string{"192.168.1.150"}
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
		"mode-help=All traffic from selected LAN devices goes through RouteFlux.",
		"default-action=direct",
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
		"split: Use separate proxy, bypass, and excluded-device lists.",
		"anti-target: deprecated alias for split with default-action=proxy and bypass-only selectors.",
		"Service presets: discord, facetime, gemini, gemini-mobile, instagram, netflix, notebooklm, notebooklm-mobile, telegram, telegram-web, twitter, whatsapp, youtube.",
		"Popular root domains like youtube.com, instagram.com, netflix.com, x.com, gemini.google.com, and notebooklm.google.com still auto-expand to the domain families they need.",
		"Gemini and NotebookLM mobile presets are broader and still best-effort because Google apps can use extra shared infrastructure and direct IPv4 endpoints.",
		"hosts: Send all traffic from selected LAN devices through RouteFlux.",
		"block-quic: when true, RouteFlux blocks proxied QUIC/UDP traffic so clients fall back to TCP; when false, QUIC is proxied normally",
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
	store.settings.Firewall.Mode = domain.FirewallModeTargets
	store.settings.Firewall.Targets = domain.FirewallSelectorSet{
		Services: []string{"youtube"},
		CIDRs:    []string{"1.1.1.1"},
		Domains:  []string{"youtube.com"},
	}
	store.settings.Firewall.Hosts = []string{"192.168.1.150"}
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
		"firewall-mode=targets",
		"firewall-default-action=direct",
		"firewall-targets=youtube, youtube.com, 1.1.1.1",
		"firewall-target-services=youtube",
		"firewall-target-domains=youtube.com",
		"firewall-target-cidrs=1.1.1.1",
		"firewall-split-proxy=",
		"firewall-split-bypass=",
		"firewall-split-excluded-sources=",
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
	if len(settings.Targets.Services) != 1 || settings.Targets.Services[0] != "youtube" {
		t.Fatalf("unexpected target services: %v", settings.Targets.Services)
	}
	if len(settings.Targets.Domains) != 1 || settings.Targets.Domains[0] != "youtube.com" {
		t.Fatalf("unexpected target domains: %v", settings.Targets.Domains)
	}
	if len(settings.Targets.CIDRs) != 1 || settings.Targets.CIDRs[0] != "1.1.1.1" {
		t.Fatalf("unexpected target cidrs: %v", settings.Targets.CIDRs)
	}
}

func TestFirewallSetAntiTargetSupportsServicesAndDomains(t *testing.T) {
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
	cmd.SetArgs([]string{"set", "anti-target", "YouTube", "YouTube.com", "1.1.1.1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute firewall set anti-target: %v", err)
	}

	if got := stdout.String(); !strings.Contains(got, "Firewall anti-targets set to youtube, youtube.com, 1.1.1.1 (deprecated: use routeflux firewall set split --bypass ...)") {
		t.Fatalf("unexpected output: %q", got)
	}

	settings, err := service.GetFirewallSettings()
	if err != nil {
		t.Fatalf("get firewall settings: %v", err)
	}
	if settings.Mode != domain.FirewallModeSplit {
		t.Fatalf("unexpected firewall mode: %q", settings.Mode)
	}
	if settings.Split.DefaultAction != domain.FirewallDefaultActionProxy {
		t.Fatalf("unexpected split default action: %q", settings.Split.DefaultAction)
	}
	if len(settings.Split.Bypass.Services) != 1 || settings.Split.Bypass.Services[0] != "youtube" {
		t.Fatalf("unexpected target services: %v", settings.Split.Bypass.Services)
	}
	if len(settings.Split.Bypass.Domains) != 1 || settings.Split.Bypass.Domains[0] != "youtube.com" {
		t.Fatalf("unexpected target domains: %v", settings.Split.Bypass.Domains)
	}
	if len(settings.Split.Bypass.CIDRs) != 1 || settings.Split.Bypass.CIDRs[0] != "1.1.1.1" {
		t.Fatalf("unexpected target cidrs: %v", settings.Split.Bypass.CIDRs)
	}
}

func TestFirewallSetSplitSupportsFlagsOnly(t *testing.T) {
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
	cmd.SetArgs([]string{"set", "split", "--proxy", "YouTube", "--exclude-host", "192.168.1.50"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute firewall set split: %v", err)
	}

	settings, err := service.GetFirewallSettings()
	if err != nil {
		t.Fatalf("get firewall settings: %v", err)
	}
	if settings.Mode != domain.FirewallModeSplit {
		t.Fatalf("unexpected firewall mode: %q", settings.Mode)
	}
	if !reflect.DeepEqual(settings.Split.Proxy.Services, []string{"youtube"}) {
		t.Fatalf("unexpected split proxy services: %+v", settings.Split.Proxy.Services)
	}
	if !reflect.DeepEqual(settings.Split.ExcludedSources, []string{"192.168.1.50"}) {
		t.Fatalf("unexpected split excluded sources: %+v", settings.Split.ExcludedSources)
	}
}

func TestFirewallDraftCommandStoresAndClearsDrafts(t *testing.T) {
	t.Parallel()

	store := &cliMemoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}
	service := app.NewService(app.Dependencies{Store: store})

	cmd := newFirewallCmd(&rootOptions{service: service})
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"draft", "targets", "youtube", "1.1.1.1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute firewall draft targets: %v", err)
	}

	settings, err := service.GetFirewallSettings()
	if err != nil {
		t.Fatalf("get firewall settings: %v", err)
	}
	if want := []string{"youtube"}; !reflect.DeepEqual(settings.ModeDrafts.Targets.TargetServices, want) {
		t.Fatalf("unexpected target draft services: %+v", settings.ModeDrafts.Targets.TargetServices)
	}
	if want := []string{"1.1.1.1"}; !reflect.DeepEqual(settings.ModeDrafts.Targets.TargetCIDRs, want) {
		t.Fatalf("unexpected target draft cidrs: %+v", settings.ModeDrafts.Targets.TargetCIDRs)
	}

	clearCmd := newFirewallCmd(&rootOptions{service: service})
	clearCmd.SetOut(new(bytes.Buffer))
	clearCmd.SetErr(new(bytes.Buffer))
	clearCmd.SetArgs([]string{"draft", "targets"})
	if err := clearCmd.Execute(); err != nil {
		t.Fatalf("execute firewall draft clear: %v", err)
	}

	settings, err = service.GetFirewallSettings()
	if err != nil {
		t.Fatalf("get firewall settings after clear: %v", err)
	}
	if !reflect.DeepEqual(settings.ModeDrafts.Targets, domain.FirewallModeDraft{}) {
		t.Fatalf("expected cleared target draft, got %+v", settings.ModeDrafts.Targets)
	}
}

func TestFirewallDraftSplitStoresFlagsOnly(t *testing.T) {
	t.Parallel()

	store := &cliMemoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}
	service := app.NewService(app.Dependencies{Store: store})

	cmd := newFirewallCmd(&rootOptions{service: service})
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"draft", "split", "--proxy", "youtube", "--exclude-host", "192.168.1.50"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute firewall draft split: %v", err)
	}

	settings, err := service.GetFirewallSettings()
	if err != nil {
		t.Fatalf("get firewall settings: %v", err)
	}
	if !reflect.DeepEqual(settings.ModeDrafts.Split.Proxy.Services, []string{"youtube"}) {
		t.Fatalf("unexpected split draft proxy services: %+v", settings.ModeDrafts.Split.Proxy.Services)
	}
	if !reflect.DeepEqual(settings.ModeDrafts.Split.ExcludedSources, []string{"192.168.1.50"}) {
		t.Fatalf("unexpected split draft excluded sources: %+v", settings.ModeDrafts.Split.ExcludedSources)
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
