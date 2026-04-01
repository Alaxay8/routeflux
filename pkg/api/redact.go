package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strings"
)

const redactedPreviewValue = "<redacted>"

var sensitivePreviewKeys = map[string]struct{}{
	"auth":          {},
	"authorization": {},
	"password":      {},
	"privatekey":    {},
	"publickey":     {},
	"secret":        {},
	"shortid":       {},
	"short_id":      {},
	"token":         {},
	"uuid":          {},
}

// XrayPreviewMetadata carries non-sensitive node hints that are useful in a
// compact preview document.
type XrayPreviewMetadata struct {
	Remark     string
	ServerName string
}

// RedactXrayPreview removes credential-like values from a rendered Xray config
// and returns a compact preview that keeps DoH and node-identifying SNI data.
func RedactXrayPreview(raw json.RawMessage, metadata *XrayPreviewMetadata) (json.RawMessage, error) {
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal xray preview: %w", err)
	}

	sanitized := sanitizePreviewValue(nil, payload)
	document, err := buildPreviewDocument(sanitized, metadata)
	if err != nil {
		return nil, err
	}

	buffer := new(bytes.Buffer)
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(document); err != nil {
		return nil, fmt.Errorf("marshal redacted xray preview: %w", err)
	}

	return json.RawMessage(bytes.TrimSpace(buffer.Bytes())), nil
}

func sanitizePreviewValue(path []string, value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, nested := range typed {
			if shouldRedactPreviewKey(path, key) {
				out[key] = redactedPreviewValue
				continue
			}
			out[key] = sanitizePreviewValue(append(path, strings.ToLower(strings.TrimSpace(key))), nested)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i := range typed {
			out[i] = sanitizePreviewValue(path, typed[i])
		}
		return out
	case string:
		if sanitized, ok := sanitizePreviewURL(typed); ok {
			return sanitized
		}
		return typed
	default:
		return value
	}
}

func shouldRedactPreviewKey(path []string, key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	if _, ok := sensitivePreviewKeys[normalized]; ok {
		return true
	}

	return normalized == "id" && slices.Contains(path, "users")
}

func sanitizePreviewURL(raw string) (string, bool) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", false
	}

	changed := false
	if parsed.User != nil {
		parsed.User = nil
		changed = true
	}
	if parsed.RawQuery != "" {
		parsed.RawQuery = ""
		changed = true
	}
	if parsed.Fragment != "" {
		parsed.Fragment = ""
		changed = true
	}

	if !changed {
		return "", false
	}

	return parsed.String(), true
}

func buildPreviewDocument(value any, metadata *XrayPreviewMetadata) (map[string]any, error) {
	root, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("xray preview must be a JSON object")
	}

	preview := make(map[string]any)

	if selected := buildSelectedNodePreview(root, metadata); len(selected) > 0 {
		preview["selected_node"] = selected
	}
	if logValue, ok := root["log"]; ok {
		preview["log"] = logValue
	}
	if dnsValue, ok := root["dns"]; ok {
		preview["dns"] = dnsValue
	}
	if inboundsValue, ok := root["inbounds"]; ok {
		preview["inbounds"] = inboundsValue
	}
	if outboundsValue := summarizePreviewOutbounds(root["outbounds"]); len(outboundsValue) > 0 {
		preview["outbounds"] = outboundsValue
	}
	if routingValue := summarizePreviewRouting(root["routing"]); len(routingValue) > 0 {
		preview["routing"] = routingValue
	}

	return preview, nil
}

func buildSelectedNodePreview(root map[string]any, metadata *XrayPreviewMetadata) map[string]any {
	selected := make(map[string]any)
	if metadata != nil && strings.TrimSpace(metadata.Remark) != "" {
		selected["remark"] = strings.TrimSpace(metadata.Remark)
	}

	serverName := ""
	if metadata != nil {
		serverName = strings.TrimSpace(metadata.ServerName)
	}
	if serverName == "" {
		serverName = discoverPreviewServerName(root)
	}
	if serverName != "" {
		selected["server_name"] = serverName
	}

	return selected
}

func summarizePreviewOutbounds(value any) []map[string]any {
	items, ok := value.([]any)
	if !ok || len(items) == 0 {
		return nil
	}

	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		outbound, ok := item.(map[string]any)
		if !ok {
			continue
		}

		summary := make(map[string]any)
		copyStringField(summary, "tag", outbound)
		copyStringField(summary, "protocol", outbound)

		if stream := summarizePreviewStreamSettings(outbound["streamSettings"]); len(stream) > 0 {
			summary["streamSettings"] = stream
		}
		if len(summary) > 0 {
			out = append(out, summary)
		}
	}

	return out
}

func summarizePreviewStreamSettings(value any) map[string]any {
	stream, ok := value.(map[string]any)
	if !ok {
		return nil
	}

	summary := make(map[string]any)
	copyStringField(summary, "network", stream)
	copyStringField(summary, "security", stream)

	if tls := summarizePreviewServerNameBlock(stream["tlsSettings"]); len(tls) > 0 {
		summary["tlsSettings"] = tls
	}
	if reality := summarizePreviewServerNameBlock(stream["realitySettings"]); len(reality) > 0 {
		summary["realitySettings"] = reality
	}

	return summary
}

func summarizePreviewServerNameBlock(value any) map[string]any {
	block, ok := value.(map[string]any)
	if !ok {
		return nil
	}

	summary := make(map[string]any)
	copyStringField(summary, "serverName", block)
	return summary
}

func summarizePreviewRouting(value any) map[string]any {
	routing, ok := value.(map[string]any)
	if !ok {
		return nil
	}

	summary := make(map[string]any)
	copyStringField(summary, "domainStrategy", routing)

	rules, ok := routing["rules"].([]any)
	if !ok || len(rules) == 0 {
		return summary
	}

	summaryRules := make([]map[string]any, 0, len(rules))
	for _, item := range rules {
		rule, ok := item.(map[string]any)
		if !ok {
			continue
		}

		entry := make(map[string]any)
		copyStringField(entry, "type", rule)
		copyStringField(entry, "outboundTag", rule)
		copyStringField(entry, "network", rule)
		copySliceField(entry, "inboundTag", rule)
		if len(entry) > 0 {
			summaryRules = append(summaryRules, entry)
		}
	}
	if len(summaryRules) > 0 {
		summary["rules"] = summaryRules
	}

	return summary
}

func discoverPreviewServerName(root map[string]any) string {
	items, ok := root["outbounds"].([]any)
	if !ok {
		return ""
	}

	fallback := ""
	for _, item := range items {
		outbound, ok := item.(map[string]any)
		if !ok {
			continue
		}

		stream, _ := outbound["streamSettings"].(map[string]any)
		serverName := extractPreviewServerName(stream)
		if serverName == "" {
			continue
		}
		if value, _ := outbound["tag"].(string); strings.TrimSpace(value) == "selected" {
			return serverName
		}
		if fallback == "" {
			fallback = serverName
		}
	}

	return fallback
}

func extractPreviewServerName(stream map[string]any) string {
	if len(stream) == 0 {
		return ""
	}

	for _, key := range []string{"realitySettings", "tlsSettings"} {
		block, _ := stream[key].(map[string]any)
		if value, _ := block["serverName"].(string); strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}

	return ""
}

func copyStringField(dst map[string]any, key string, src map[string]any) {
	value, _ := src[key].(string)
	value = strings.TrimSpace(value)
	if value != "" {
		dst[key] = value
	}
}

func copySliceField(dst map[string]any, key string, src map[string]any) {
	values, ok := src[key].([]any)
	if !ok || len(values) == 0 {
		return
	}

	dst[key] = values
}
