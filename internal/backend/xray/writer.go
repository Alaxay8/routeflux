package xray

import (
	"context"
	"fmt"

	"github.com/Alaxay8/routeflux/internal/backend"
	"github.com/Alaxay8/routeflux/internal/store"
)

// FileWriter persists generated Xray configs.
type FileWriter struct {
	Path string
}

// Write saves the rendered config atomically.
func (w FileWriter) Write(data []byte) error {
	if err := store.AtomicWriteJSON(w.Path, jsonRaw(data)); err != nil {
		return fmt.Errorf("write xray config: %w", err)
	}
	return nil
}

type jsonRaw []byte

func (r jsonRaw) MarshalJSON() ([]byte, error) {
	return r, nil
}

// RuntimeBackend applies generated config and controls the Xray service.
type RuntimeBackend struct {
	generator  Generator
	writer     FileWriter
	controller backend.ServiceController
}

// NewRuntimeBackend creates an operational Xray backend.
func NewRuntimeBackend(configPath string, controller backend.ServiceController) RuntimeBackend {
	return RuntimeBackend{
		generator:  NewGenerator(),
		writer:     FileWriter{Path: configPath},
		controller: controller,
	}
}

// GenerateConfig renders the backend configuration.
func (b RuntimeBackend) GenerateConfig(req backend.ConfigRequest) ([]byte, error) {
	return b.generator.Generate(req)
}

// ApplyConfig writes the config and reloads the service.
func (b RuntimeBackend) ApplyConfig(ctx context.Context, req backend.ConfigRequest) error {
	rendered, err := b.GenerateConfig(req)
	if err != nil {
		return err
	}
	if err := b.writer.Write(rendered); err != nil {
		return err
	}
	if b.controller != nil {
		return b.controller.Reload(ctx)
	}
	return nil
}

// Start starts the Xray runtime.
func (b RuntimeBackend) Start(ctx context.Context) error {
	if b.controller == nil {
		return nil
	}
	return b.controller.Start(ctx)
}

// Stop stops the Xray runtime.
func (b RuntimeBackend) Stop(ctx context.Context) error {
	if b.controller == nil {
		return nil
	}
	return b.controller.Stop(ctx)
}

// Reload reloads the Xray runtime.
func (b RuntimeBackend) Reload(ctx context.Context) error {
	if b.controller == nil {
		return nil
	}
	return b.controller.Reload(ctx)
}

// Status returns the service status if a controller is configured.
func (b RuntimeBackend) Status(ctx context.Context) (backend.RuntimeStatus, error) {
	if b.controller == nil {
		return backend.RuntimeStatus{}, nil
	}
	return b.controller.Status(ctx)
}
