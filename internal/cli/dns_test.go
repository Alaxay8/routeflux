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
	cmd.SetArgs([]string{"set", "mode", "split"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute dns set mode: %v", err)
	}

	if store.settings.DNS.Mode != domain.DNSModeSplit {
		t.Fatalf("unexpected dns mode: %s", store.settings.DNS.Mode)
	}
	if got := stdout.String(); !strings.Contains(got, "Updated dns.mode=split") {
		t.Fatalf("unexpected output: %q", got)
	}

	stdout.Reset()
	cmd.SetArgs([]string{"set", "transport", "doh"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute dns set transport: %v", err)
	}

	if store.settings.DNS.Transport != domain.DNSTransportDoH {
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
		"system: RouteFlux does not touch DNS settings.",
		"remote: Send all DNS requests to the DNS servers you choose.",
		"split: Keep local home-network names local",
		"doh: DNS over HTTPS.",
		"dot: DNS over TLS.",
	}
	for _, want := range wants {
		if !strings.Contains(output, want) {
			t.Fatalf("dns explain missing %q\n%s", want, output)
		}
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
