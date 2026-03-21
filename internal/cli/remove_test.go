package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Alaxay8/routeflux/internal/app"
	"github.com/Alaxay8/routeflux/internal/domain"
)

func TestRemoveCommandDeletesSubscription(t *testing.T) {
	t.Parallel()

	store := &cliMemoryStore{
		subs: []domain.Subscription{
			{ID: "sub-1", DisplayName: "Alpha"},
			{ID: "sub-2", DisplayName: "Beta"},
		},
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}

	cmd := newRemoveCmd(&rootOptions{service: app.NewService(app.Dependencies{Store: store})})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"sub-1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute remove: %v", err)
	}

	if got := stdout.String(); !strings.Contains(got, "Removed subscription sub-1") {
		t.Fatalf("unexpected output: %q", got)
	}
	if len(store.subs) != 1 || store.subs[0].ID != "sub-2" {
		t.Fatalf("unexpected subscriptions after removal: %+v", store.subs)
	}
}
