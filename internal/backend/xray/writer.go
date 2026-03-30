package xray

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

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
	tester     ConfigTester
	backupPath string
	logger     *slog.Logger
}

// NewRuntimeBackend creates an operational Xray backend.
func NewRuntimeBackend(configPath string, controller backend.ServiceController) RuntimeBackend {
	return RuntimeBackend{
		generator:  NewGenerator(),
		writer:     FileWriter{Path: configPath},
		controller: controller,
		tester:     NewCommandTester(),
		backupPath: configPath + ".last-known-good",
	}
}

// WithLogger configures an optional logger for backend runtime events.
func (b RuntimeBackend) WithLogger(logger *slog.Logger) RuntimeBackend {
	b.logger = logger
	return b
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
	b.logInfo("validate xray config", "config_path", b.writer.Path)
	if err := b.validateConfig(ctx, rendered); err != nil {
		b.logWarn("xray config validation failed", "config_path", b.writer.Path, "error", err.Error())
		return err
	}
	b.logInfo("xray config validated", "config_path", b.writer.Path)

	rollbackData, hasRollback, err := b.rollbackData()
	if err != nil {
		return err
	}
	if hasRollback {
		if err := writeRawConfig(b.backupPath, rollbackData); err != nil {
			return fmt.Errorf("write xray last-known-good config: %w", err)
		}
	}
	if err := b.writer.Write(rendered); err != nil {
		return err
	}
	if b.controller != nil {
		b.logInfo("reload xray service", "config_path", b.writer.Path)
		if err := b.controller.Reload(ctx); err != nil {
			b.logWarn("xray service reload failed", "config_path", b.writer.Path, "error", err.Error())
			if rollbackErr := b.rollbackReload(ctx, rollbackData, hasRollback); rollbackErr != nil {
				b.logWarn("xray rollback reload failed", "config_path", b.writer.Path, "error", rollbackErr.Error())
				return fmt.Errorf("reload xray service: %w; rollback xray service reload: %v", err, rollbackErr)
			}
			b.logWarn("xray config rolled back", "config_path", b.writer.Path, "backup_path", b.backupPath)
			return fmt.Errorf("reload xray service: %w", err)
		}
	}
	if err := writeRawConfig(b.backupPath, rendered); err != nil {
		return fmt.Errorf("write xray last-known-good config: %w", err)
	}
	b.logInfo("xray config applied", "config_path", b.writer.Path, "backup_path", b.backupPath)
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
		return backend.RuntimeStatus{ConfigPath: b.writer.Path}, nil
	}

	status, err := b.controller.Status(ctx)
	status.ConfigPath = b.writer.Path
	return status, err
}

func (b RuntimeBackend) validateConfig(ctx context.Context, rendered []byte) error {
	if b.tester == nil {
		return nil
	}

	dir := filepath.Dir(b.writer.Path)
	if err := os.MkdirAll(dir, store.PrivateDirPerm); err != nil {
		return fmt.Errorf("create xray config directory: %w", err)
	}

	tmp, err := os.CreateTemp(dir, candidateConfigPattern(filepath.Base(b.writer.Path)))
	if err != nil {
		return fmt.Errorf("create xray candidate config: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmp.Write(rendered); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write xray candidate config: %w", err)
	}
	if err := tmp.Chmod(store.SecretFilePerm); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod xray candidate config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close xray candidate config: %w", err)
	}

	if err := b.tester.Test(ctx, tmpPath); err != nil {
		return fmt.Errorf("test xray config: %w", err)
	}

	return nil
}

func candidateConfigPattern(name string) string {
	ext := filepath.Ext(name)
	if ext == "" {
		return name + ".candidate-*"
	}

	stem := strings.TrimSuffix(name, ext)
	return stem + ".candidate-*" + ext
}

func (b RuntimeBackend) rollbackData() ([]byte, bool, error) {
	if data, err := os.ReadFile(b.backupPath); err == nil {
		return data, true, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, false, fmt.Errorf("read xray last-known-good config: %w", err)
	}

	if data, err := os.ReadFile(b.writer.Path); err == nil {
		return data, true, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, false, fmt.Errorf("read current xray config: %w", err)
	}

	return nil, false, nil
}

func (b RuntimeBackend) rollbackReload(ctx context.Context, rollbackData []byte, hasRollback bool) error {
	if hasRollback {
		if err := writeRawConfig(b.writer.Path, rollbackData); err != nil {
			return fmt.Errorf("restore xray config: %w", err)
		}
	} else if err := os.Remove(b.writer.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove failed xray config: %w", err)
	}

	if b.controller == nil {
		return nil
	}

	if err := b.controller.Reload(ctx); err != nil {
		return err
	}

	return nil
}

func writeRawConfig(path string, data []byte) error {
	if path == "" {
		return nil
	}
	if err := store.AtomicWriteJSON(path, jsonRaw(data)); err != nil {
		return fmt.Errorf("write xray config: %w", err)
	}
	return nil
}

func (b RuntimeBackend) logInfo(msg string, args ...any) {
	if b.logger != nil {
		b.logger.Info(msg, args...)
	}
}

func (b RuntimeBackend) logWarn(msg string, args ...any) {
	if b.logger != nil {
		b.logger.Warn(msg, args...)
	}
}
