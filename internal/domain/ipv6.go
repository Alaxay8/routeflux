package domain

// IPv6Status reports the current router IPv6 state relevant to RouteFlux.
type IPv6Status struct {
	Available          bool     `json:"available"`
	ConfigPath         string   `json:"config_path,omitempty"`
	PersistentDisabled bool     `json:"persistent_disabled"`
	RuntimeDisabled    bool     `json:"runtime_disabled"`
	EnabledInterfaces  []string `json:"enabled_interfaces,omitempty"`
}
