package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Alaxay8/routeflux/internal/app"
	"github.com/Alaxay8/routeflux/internal/domain"
)

func TestServicesSetGetAndFirewallTargetsUseCustomAlias(t *testing.T) {
	t.Parallel()

	store := &cliMemoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}
	service := app.NewService(app.Dependencies{Store: store})

	setCmd := newServicesCmd(&rootOptions{service: service})
	setCmd.SetOut(new(bytes.Buffer))
	setCmd.SetErr(new(bytes.Buffer))
	setCmd.SetArgs([]string{"set", "openai", "openai.com", "chatgpt.com", "104.18.0.0/15"})
	if err := setCmd.Execute(); err != nil {
		t.Fatalf("execute services set: %v", err)
	}

	getCmd := newServicesCmd(&rootOptions{service: service})
	var stdout bytes.Buffer
	getCmd.SetOut(&stdout)
	getCmd.SetErr(new(bytes.Buffer))
	getCmd.SetArgs([]string{"get", "openai"})
	if err := getCmd.Execute(); err != nil {
		t.Fatalf("execute services get: %v", err)
	}

	output := stdout.String()
	for _, want := range []string{
		"name=openai",
		"source=custom",
		"readonly=false",
		"domains=openai.com, chatgpt.com",
		"cidrs=104.18.0.0/15",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("services get missing %q\n%s", want, output)
		}
	}

	firewallCmd := newFirewallCmd(&rootOptions{service: service})
	firewallCmd.SetOut(new(bytes.Buffer))
	firewallCmd.SetErr(new(bytes.Buffer))
	firewallCmd.SetArgs([]string{"set", "targets", "openai"})
	if err := firewallCmd.Execute(); err != nil {
		t.Fatalf("execute firewall set targets: %v", err)
	}

	settings, err := service.GetFirewallSettings()
	if err != nil {
		t.Fatalf("get firewall settings: %v", err)
	}
	if len(settings.TargetServices) != 1 || settings.TargetServices[0] != "openai" {
		t.Fatalf("unexpected target services: %+v", settings.TargetServices)
	}
}

func TestServicesDeleteRejectsBuiltinName(t *testing.T) {
	t.Parallel()

	store := &cliMemoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}
	service := app.NewService(app.Dependencies{Store: store})

	cmd := newServicesCmd(&rootOptions{service: service})
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"delete", "youtube"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected delete builtin service to fail")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "readonly") {
		t.Fatalf("unexpected error: %v", err)
	}
}
