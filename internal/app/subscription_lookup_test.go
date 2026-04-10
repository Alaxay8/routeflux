package app

import (
	"strings"
	"testing"

	"github.com/Alaxay8/routeflux/internal/domain"
)

func TestSubscriptionByIDAcceptsUniquePrefix(t *testing.T) {
	t.Parallel()

	service := NewService(Dependencies{
		Store: &memoryStore{
			subs: []domain.Subscription{
				{ID: "sub-1234567890ab", DisplayName: "Alpha"},
				{ID: "sub-fedcba098765", DisplayName: "Beta"},
			},
		},
	})

	sub, err := service.subscriptionByID("sub-1234")
	if err != nil {
		t.Fatalf("lookup by prefix: %v", err)
	}
	if sub.ID != "sub-1234567890ab" {
		t.Fatalf("unexpected subscription: %+v", sub)
	}
}

func TestSubscriptionByIDRejectsAmbiguousPrefix(t *testing.T) {
	t.Parallel()

	service := NewService(Dependencies{
		Store: &memoryStore{
			subs: []domain.Subscription{
				{ID: "sub-1234567890ab", DisplayName: "Alpha"},
				{ID: "sub-1234fedcba98", DisplayName: "Beta"},
			},
		},
	})

	_, err := service.subscriptionByID("sub-1234")
	if err == nil {
		t.Fatal("expected ambiguous prefix error")
	}
	if !strings.Contains(err.Error(), `subscription "sub-1234" is ambiguous`) {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "sub-1234567890ab") || !strings.Contains(err.Error(), "sub-1234fedcba98") {
		t.Fatalf("expected matching ids in error, got: %v", err)
	}
}

func TestSubscriptionByIDPrefersExactIDOverPrefixMatches(t *testing.T) {
	t.Parallel()

	service := NewService(Dependencies{
		Store: &memoryStore{
			subs: []domain.Subscription{
				{ID: "sub-1234", DisplayName: "Exact"},
				{ID: "sub-1234567890ab", DisplayName: "Longer"},
			},
		},
	})

	sub, err := service.subscriptionByID("sub-1234")
	if err != nil {
		t.Fatalf("lookup exact id: %v", err)
	}
	if sub.ID != "sub-1234" {
		t.Fatalf("unexpected subscription: %+v", sub)
	}
}
