package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Alaxay8/routeflux/internal/app"
	"github.com/Alaxay8/routeflux/internal/domain"
)

func TestZapretGetExpandsLegacyAliasesToDomains(t *testing.T) {
	t.Parallel()

	settings := domain.DefaultSettings()
	settings.Zapret = domain.ZapretSettings{
		Enabled: true,
		Selectors: domain.FirewallSelectorSet{
			Services: []string{"youtube"},
			Domains:  []string{"example.com"},
		},
		FailbackSuccessThreshold: 4,
	}

	cmd := newZapretCmd(&rootOptions{service: app.NewService(app.Dependencies{Store: &cliMemoryStore{
		settings: settings,
		state:    domain.DefaultRuntimeState(),
	}})})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"get"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute zapret get: %v", err)
	}

	output := stdout.String()
	for _, want := range []string{
		"enabled=true",
		"domains=example.com, ggpht.com, googlevideo.com, youtu.be, youtube-nocookie.com, youtube.com, youtube.googleapis.com, youtubei.googleapis.com, ytimg.com",
		"failback-success-threshold=4",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("zapret get missing %q\n%s", want, output)
		}
	}

	if strings.Contains(output, "\nservices=") {
		t.Fatalf("zapret get must not render services anymore\n%s", output)
	}
}

func TestZapretHelpDoesNotExposeResourcesCommand(t *testing.T) {
	t.Parallel()

	cmd := newZapretCmd(&rootOptions{service: app.NewService(app.Dependencies{Store: &cliMemoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}})})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute zapret --help: %v", err)
	}

	output := stdout.String()
	if strings.Contains(output, "resources") {
		t.Fatalf("zapret help must not mention resources anymore\n%s", output)
	}
	if !strings.Contains(output, "set") {
		t.Fatalf("zapret help must keep the set command\n%s", output)
	}
}
