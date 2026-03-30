package domain

import "time"

// SourceType identifies how a subscription was added.
type SourceType string

const (
	// SourceTypeURL represents a remote HTTP or HTTPS subscription.
	SourceTypeURL SourceType = "url"
	// SourceTypeRaw represents raw link or raw subscription content.
	SourceTypeRaw SourceType = "raw"
)

// ProviderNameSource identifies where the current provider name came from.
type ProviderNameSource string

const (
	// ProviderNameSourceDefault means the generic fallback name is in use.
	ProviderNameSourceDefault ProviderNameSource = "default"
	// ProviderNameSourceURL means the name was derived from the subscription URL.
	ProviderNameSourceURL ProviderNameSource = "url"
	// ProviderNameSourceHeader means the name came from subscription response metadata.
	ProviderNameSourceHeader ProviderNameSource = "header"
	// ProviderNameSourceManual means the user explicitly set or renamed the name.
	ProviderNameSourceManual ProviderNameSource = "manual"
)

// Subscription stores a provider payload and its normalized nodes.
type Subscription struct {
	ID                 string             `json:"id"`
	SourceType         SourceType         `json:"source_type"`
	Source             string             `json:"source"`
	ProviderName       string             `json:"provider_name"`
	ProviderNameSource ProviderNameSource `json:"provider_name_source,omitempty"`
	DisplayName        string             `json:"display_name"`
	LastUpdatedAt      time.Time          `json:"last_updated_at"`
	ExpiresAt          *time.Time         `json:"expires_at,omitempty"`
	RefreshInterval    Duration           `json:"refresh_interval"`
	LastError          string             `json:"last_error"`
	ParserStatus       string             `json:"parser_status"`
	Nodes              []Node             `json:"nodes"`
}

// NodeByID looks up a node inside the subscription.
func (s Subscription) NodeByID(id string) (Node, bool) {
	for _, node := range s.Nodes {
		if node.ID == id {
			return node, true
		}
	}

	return Node{}, false
}
