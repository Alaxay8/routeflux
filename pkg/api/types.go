package api

import "github.com/Alaxay8/routeflux/internal/domain"

// SubscriptionResponse is the API-safe shape for a stored subscription.
type SubscriptionResponse struct {
	ID            string        `json:"id"`
	ProviderName  string        `json:"provider_name"`
	DisplayName   string        `json:"display_name"`
	LastUpdatedAt string        `json:"last_updated_at"`
	ParserStatus  string        `json:"parser_status"`
	LastError     string        `json:"last_error"`
	NodeCount     int           `json:"node_count"`
	RefreshEvery  string        `json:"refresh_every"`
	Nodes         []domain.Node `json:"nodes,omitempty"`
}
