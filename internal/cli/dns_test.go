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
		"Preset, not a fifth mode.",
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
		"default     Apply the Recommended DNS preset",
		"routeflux dns default",
		"routeflux dns set mode system",
		"Recommended DNS preset (not a mode): routeflux dns default.",
	}
	for _, want := range wants {
		if !strings.Contains(output, want) {
			t.Fatalf("dns help missing %q\n%s", want, output)
		}
	}
	unwanted := []string{
		"apply       Replace the full DNS profile in one step",
		"routeflux dns apply",
	}
	for _, item := range unwanted {
		if strings.Contains(output, item) {
			t.Fatalf("dns help unexpectedly contains %q\n%s", item, output)
		}
	}
}

func TestDNSSetHelpFocusesOnCommonPath(t *testing.T) {
	t.Parallel()

	cmd := newDNSCmd(&rootOptions{service: app.NewService(app.Dependencies{Store: &cliMemoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}})})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"set", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute dns set help: %v", err)
	}

	output := stdout.String()
	wants := []string{
		"Recommended start: routeflux dns default",
		"default: apply the Recommended DNS preset (preset, not a mode)",
		"system: leave DNS as it is",
		"remote: send all DNS to the servers you choose",
		"split: keep local names local and send internet DNS to the servers you choose",
		"disabled: do not write RouteFlux DNS settings into the Xray config",
	}
	for _, want := range wants {
		if !strings.Contains(output, want) {
			t.Fatalf("dns set help missing %q\n%s", want, output)
		}
	}
	if strings.Contains(output, "Simple meaning:") {
		t.Fatalf("dns set help should not repeat the old simple meaning block\n%s", output)
	}
}

func TestDNSApplyHelpRemainsAvailableDirectly(t *testing.T) {
	t.Parallel()

	cmd := newDNSCmd(&rootOptions{service: app.NewService(app.Dependencies{Store: &cliMemoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}})})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"apply", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute dns apply help: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Apply a complete DNS profile atomically.") {
		t.Fatalf("dns apply help missing command summary\n%s", output)
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
	if !strings.Contains(stdout.String(), "Applied the Recommended DNS preset.") {
		t.Fatalf("unexpected output: %q", stdout.String())
	}
}

func TestDNSApplyReplacesProfileAtomically(t *testing.T) {
	t.Parallel()

	store := &cliMemoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}

	cmd := newDNSCmd(&rootOptions{service: app.NewService(app.Dependencies{Store: store})})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{
		"apply",
		"--mode=split",
		"--transport=doh",
		"--servers=1.1.1.1,1.0.0.1",
		"--bootstrap=",
		"--direct-domains=domain:lan,full:router.lan",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute dns apply: %v", err)
	}

	if store.settings.DNS.Mode != domain.DNSModeSplit || store.settings.DNS.Transport != domain.DNSTransportDoH {
		t.Fatalf("unexpected dns profile: %+v", store.settings.DNS)
	}
	if len(store.settings.DNS.Servers) != 2 || store.settings.DNS.Servers[0] != "1.1.1.1" || store.settings.DNS.Servers[1] != "1.0.0.1" {
		t.Fatalf("unexpected dns servers: %+v", store.settings.DNS.Servers)
	}
	if len(store.settings.DNS.Bootstrap) != 0 {
		t.Fatalf("expected bootstrap to be cleared, got %+v", store.settings.DNS.Bootstrap)
	}
	if !strings.Contains(stdout.String(), "Applied DNS settings atomically.") {
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
	if !strings.Contains(stdout.String(), "Applied the Recommended DNS preset.") {
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

	if !strings.Contains(stdout.String(), "profile=Recommended DNS preset") {
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
