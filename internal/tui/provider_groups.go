package tui

import (
	"fmt"
	"net"
	"slices"
	"strings"
	"unicode"

	"github.com/Alaxay8/routeflux/internal/domain"
)

type providerGroup struct {
	Key           string
	Title         string
	Subscriptions []providerSubscription
	TotalNodes    int
}

type providerSubscription struct {
	Subscription domain.Subscription
	Label        string
}

func buildProviderGroups(subscriptions []domain.Subscription) []providerGroup {
	if len(subscriptions) == 0 {
		return nil
	}

	groupsByKey := make(map[string]*providerGroup, len(subscriptions))
	for _, sub := range subscriptions {
		key := providerKey(sub)
		group := groupsByKey[key]
		if group == nil {
			group = &providerGroup{
				Key:   key,
				Title: providerTitle(sub),
			}
			groupsByKey[key] = group
		}

		group.Subscriptions = append(group.Subscriptions, providerSubscription{
			Subscription: sub,
			Label:        profileLabel(sub),
		})
		group.TotalNodes += len(sub.Nodes)
	}

	groups := make([]providerGroup, 0, len(groupsByKey))
	for _, group := range groupsByKey {
		ensureProfileLabels(group)
		slices.SortFunc(group.Subscriptions, func(a, b providerSubscription) int {
			return strings.Compare(strings.ToLower(a.Label), strings.ToLower(b.Label))
		})
		groups = append(groups, *group)
	}

	slices.SortFunc(groups, func(a, b providerGroup) int {
		return strings.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title))
	})

	return groups
}

func providerKey(sub domain.Subscription) string {
	if value := strings.TrimSpace(sub.ProviderName); value != "" {
		return strings.ToLower(value)
	}
	if value := strings.TrimSpace(sub.DisplayName); value != "" {
		return strings.ToLower(value)
	}
	return sub.ID
}

func providerTitle(sub domain.Subscription) string {
	if value := strings.TrimSpace(sub.ProviderName); value != "" {
		return humanizeProviderName(value)
	}
	if value := strings.TrimSpace(sub.DisplayName); value != "" {
		return humanizeProviderName(value)
	}
	return "Imported VPN"
}

func profileLabel(sub domain.Subscription) string {
	label := strings.TrimSpace(sub.DisplayName)
	if label == "" || strings.EqualFold(label, strings.TrimSpace(sub.ProviderName)) {
		return ""
	}
	return label
}

func ensureProfileLabels(group *providerGroup) {
	used := make(map[string]int, len(group.Subscriptions))
	for idx := range group.Subscriptions {
		label := strings.TrimSpace(group.Subscriptions[idx].Label)
		if label == "" {
			label = fmt.Sprintf("Profile %d", idx+1)
		}

		count := used[strings.ToLower(label)]
		if count > 0 {
			label = fmt.Sprintf("%s %d", label, count+1)
		}
		used[strings.ToLower(group.Subscriptions[idx].Label)]++
		used[strings.ToLower(label)]++
		group.Subscriptions[idx].Label = label
	}
}

func humanizeProviderName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "Imported VPN"
	}

	if isDomainLike(value) {
		label := strings.ToLower(strings.Split(value, ".")[0])
		for _, prefix := range []string{"www-", "www", "sub-", "sub", "api-", "api", "conn"} {
			if strings.HasPrefix(label, prefix) && len(label) > len(prefix)+2 {
				label = strings.TrimPrefix(label, prefix)
				break
			}
		}
		label = strings.NewReplacer("-", " ", "_", " ").Replace(label)
		label = titleWords(label)
		if !strings.Contains(strings.ToLower(label), "vpn") {
			label += " VPN"
		}
		return strings.TrimSpace(label)
	}

	return value
}

func isDomainLike(value string) bool {
	host := strings.TrimSpace(value)
	if strings.Contains(host, "://") {
		return false
	}
	if strings.Contains(host, " ") {
		return false
	}
	return strings.Contains(host, ".") && net.ParseIP(host) == nil
}

func titleWords(value string) string {
	parts := strings.Fields(strings.TrimSpace(value))
	for idx, part := range parts {
		runes := []rune(strings.ToLower(part))
		if len(runes) == 0 {
			continue
		}
		runes[0] = unicode.ToUpper(runes[0])
		parts[idx] = string(runes)
	}
	return strings.Join(parts, " ")
}

func (m model) currentProvider() (providerGroup, bool) {
	if len(m.providers) == 0 {
		return providerGroup{}, false
	}
	return m.providers[m.selectedProvider], true
}

func (m model) currentProfile() (providerSubscription, bool) {
	provider, ok := m.currentProvider()
	if !ok || len(provider.Subscriptions) == 0 {
		return providerSubscription{}, false
	}
	return provider.Subscriptions[m.selectedProfile], true
}

func (m model) currentSubscription() (domain.Subscription, bool) {
	profile, ok := m.currentProfile()
	if !ok {
		return domain.Subscription{}, false
	}
	return profile.Subscription, true
}

func (m model) currentNode() (domain.Node, bool) {
	sub, ok := m.currentSubscription()
	if !ok || len(sub.Nodes) == 0 {
		return domain.Node{}, false
	}
	return sub.Nodes[m.selectedNode], true
}
