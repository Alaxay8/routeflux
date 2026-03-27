package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Alaxay8/routeflux/internal/app"
	"github.com/Alaxay8/routeflux/internal/domain"
)

func TestDNSSetCommandUpdatesSettings(t *testing.T) {
	t.Parallel()

	store := &cliMemoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}

	cmd := newDNSCmd(&rootOptions{service: app.NewService(app.Dependencies{Store: store})})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"set", "mode", "system"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute dns set mode: %v", err)
	}

	if store.settings.DNS.Mode != domain.DNSModeSystem {
		t.Fatalf("unexpected dns mode: %s", store.settings.DNS.Mode)
	}
	if got := stdout.String(); !strings.Contains(got, "Updated dns.mode=system") {
		t.Fatalf("unexpected output: %q", got)
	}

	stdout.Reset()
	cmd.SetArgs([]string{"set", "transport", "plain"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute dns set transport: %v", err)
	}

	if store.settings.DNS.Transport != domain.DNSTransportPlain {
		t.Fatalf("unexpected dns transport: %s", store.settings.DNS.Transport)
	}
}

func TestDNSExplainCommandOutputsFriendlyGuide(t *testing.T) {
	t.Parallel()

	cmd := newDNSCmd(&rootOptions{service: app.NewService(app.Dependencies{Store: &cliMemoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}})})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"explain"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute dns explain: %v", err)
	}

	output := stdout.String()
	wants := []string{
		"system: Leave DNS as it is.",
		"remote: Send every DNS request to the DNS servers you choose.",
		"split: Keep local names on the router",
		"doh: encrypted DNS over HTTPS.",
		"routeflux dns set default",
	}
	for _, want := range wants {
		if !strings.Contains(output, want) {
			t.Fatalf("dns explain missing %q\n%s", want, output)
		}
	}
}

func TestDNSHelpIncludesDefaultCommand(t *testing.T) {
	t.Parallel()

	cmd := newDNSCmd(&rootOptions{service: app.NewService(app.Dependencies{Store: &cliMemoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}})})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute dns help: %v", err)
	}

	output := stdout.String()
	wants := []string{
		"default     Apply the RouteFlux recommended DNS profile",
		"routeflux dns default",
		"routeflux dns set default",
	}
	for _, want := range wants {
		if !strings.Contains(output, want) {
			t.Fatalf("dns help missing %q\n%s", want, output)
		}
	}
}

func TestDNSSetDefaultAppliesRecommendedProfile(t *testing.T) {
	t.Parallel()

	store := &cliMemoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}
	store.settings.DNS.Mode = domain.DNSModeSystem
	store.settings.DNS.Transport = domain.DNSTransportPlain
	store.settings.DNS.Servers = nil
	store.settings.DNS.Bootstrap = []string{"9.9.9.9"}
	store.settings.DNS.DirectDomains = nil

	cmd := newDNSCmd(&rootOptions{service: app.NewService(app.Dependencies{Store: store})})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"set", "default"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute dns set default: %v", err)
	}

	want := domain.DefaultDNSSettings()
	if store.settings.DNS.Mode != want.Mode || store.settings.DNS.Transport != want.Transport {
		t.Fatalf("unexpected dns profile: %+v", store.settings.DNS)
	}
	if !strings.Contains(stdout.String(), "Applied the RouteFlux default DNS profile.") {
		t.Fatalf("unexpected output: %q", stdout.String())
	}
}

func TestDNSDefaultCommandAppliesRecommendedProfile(t *testing.T) {
	t.Parallel()

	store := &cliMemoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}
	store.settings.DNS.Mode = domain.DNSModeSystem
	store.settings.DNS.Transport = domain.DNSTransportPlain
	store.settings.DNS.Servers = nil
	store.settings.DNS.Bootstrap = []string{"9.9.9.9"}
	store.settings.DNS.DirectDomains = nil

	cmd := newDNSCmd(&rootOptions{service: app.NewService(app.Dependencies{Store: store})})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"default"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute dns default: %v", err)
	}

	want := domain.DefaultDNSSettings()
	if store.settings.DNS.Mode != want.Mode || store.settings.DNS.Transport != want.Transport {
		t.Fatalf("unexpected dns profile: %+v", store.settings.DNS)
	}
	if !strings.Contains(stdout.String(), "Applied the RouteFlux default DNS profile.") {
		t.Fatalf("unexpected output: %q", stdout.String())
	}
}

func TestDNSGetShowsCurrentValuesAndMeaning(t *testing.T) {
	t.Parallel()

	store := &cliMemoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}
	store.settings.DNS.Mode = domain.DNSModeSplit
	store.settings.DNS.Transport = domain.DNSTransportDoH
	store.settings.DNS.Servers = []string{"dns.google", "1.1.1.1"}
	store.settings.DNS.Bootstrap = []string{"9.9.9.9"}
	store.settings.DNS.DirectDomains = []string{"domain:lan", "full:router.lan"}

	cmd := newDNSCmd(&rootOptions{service: app.NewService(app.Dependencies{Store: store})})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"get"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute dns get: %v", err)
	}

	output := stdout.String()
	wants := []string{
		"mode=split",
		"mode-help=Keep local home names on the router",
		"transport=doh",
		"transport-help=Encrypted DNS over HTTPS.",
		"servers=dns.google, 1.1.1.1",
		"bootstrap=9.9.9.9",
		"direct-domains=domain:lan, full:router.lan",
	}
	for _, want := range wants {
		if !strings.Contains(output, want) {
			t.Fatalf("dns get missing %q\n%s", want, output)
		}
	}

	defaultStore := &cliMemoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}
	defaultCmd := newDNSCmd(&rootOptions{service: app.NewService(app.Dependencies{Store: defaultStore})})
	stdout.Reset()
	defaultCmd.SetOut(&stdout)
	defaultCmd.SetErr(new(bytes.Buffer))
	defaultCmd.SetArgs([]string{"get"})

	if err := defaultCmd.Execute(); err != nil {
		t.Fatalf("execute default dns get: %v", err)
	}

	if !strings.Contains(stdout.String(), "profile=routeflux-default") {
		t.Fatalf("default dns get missing profile label\n%s", stdout.String())
	}
}

func TestRootHelpIncludesDNSCommand(t *testing.T) {
	t.Parallel()

	cmd := newRootCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute root help: %v", err)
	}

	if got := stdout.String(); !strings.Contains(got, "dns         Easy DNS settings for RouteFlux") {
		t.Fatalf("root help missing dns command\n%s", got)
	}
}
