package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Alaxay8/routeflux/internal/app"
	"github.com/Alaxay8/routeflux/internal/domain"
)

func TestRefreshCommandRequiresSubscriptionOrAll(t *testing.T) {
	t.Parallel()

	cmd := newRefreshCmd(&rootOptions{service: app.NewService(app.Dependencies{Store: &cliMemoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}})})
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(nil)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected refresh without flags to fail")
	}
	if !strings.Contains(err.Error(), "use --all or --subscription") {
		t.Fatalf("unexpected error: %v", err)
	}
}
