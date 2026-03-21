package backend

import (
	"context"

	"github.com/Alaxay8/routeflux/internal/domain"
)

// ConfigRequest defines the inputs required to build a backend config.
type ConfigRequest struct {
	Mode             domain.SelectionMode
	Nodes            []domain.Node
	SelectedNodeID   string
	LogLevel         string
	SOCKSPort        int
	HTTPPort         int
	TransparentProxy bool
	TransparentPort  int
}

// RuntimeStatus describes backend runtime state.
type RuntimeStatus struct {
	Running      bool   `json:"running"`
	ConfigPath   string `json:"config_path"`
	ServiceState string `json:"service_state"`
}

// ServiceController abstracts runtime service management.
type ServiceController interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Reload(ctx context.Context) error
	Status(ctx context.Context) (RuntimeStatus, error)
}

// Backend abstracts a runtime backend such as Xray.
type Backend interface {
	GenerateConfig(req ConfigRequest) ([]byte, error)
	ApplyConfig(ctx context.Context, req ConfigRequest) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Reload(ctx context.Context) error
	Status(ctx context.Context) (RuntimeStatus, error)
}
