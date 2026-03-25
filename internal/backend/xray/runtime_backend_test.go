package xray

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Alaxay8/routeflux/internal/backend"
	"github.com/Alaxay8/routeflux/internal/domain"
)

func TestRuntimeBackendApplyConfigValidationFailureKeepsLiveConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	livePath := filepath.Join(dir, "config.json")
	liveConfig := []byte("{\"log\":{\"loglevel\":\"warning\"}}\n")
	if err := os.WriteFile(livePath, liveConfig, 0o644); err != nil {
		t.Fatalf("write live config: %v", err)
	}

	tester := &recordingConfigTester{err: errors.New("invalid config")}
	controller := &scriptedController{}
	runtimeBackend := NewRuntimeBackend(livePath, controller)
	runtimeBackend.tester = tester

	err := runtimeBackend.ApplyConfig(context.Background(), testConfigRequest())
	if err == nil {
		t.Fatal("expected validation failure")
	}
	if !strings.Contains(err.Error(), "test xray config") {
		t.Fatalf("unexpected error: %v", err)
	}
	if controller.reloadCalls != 0 {
		t.Fatalf("expected no reloads, got %d", controller.reloadCalls)
	}
	if len(tester.configPaths) != 1 {
		t.Fatalf("expected one validation attempt, got %d", len(tester.configPaths))
	}
	if got := tester.configPaths[0]; filepath.Ext(got) != ".json" {
		t.Fatalf("expected candidate config to keep .json extension, got %q", got)
	}

	got, err := os.ReadFile(livePath)
	if err != nil {
		t.Fatalf("read live config: %v", err)
	}
	if string(got) != string(liveConfig) {
		t.Fatalf("live config changed on validation failure\nwant:\n%s\ngot:\n%s", liveConfig, got)
	}
}

func TestCandidateConfigPatternKeepsExtension(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
	}{
		{name: "config.json", want: "config.candidate-*.json"},
		{name: "config", want: "config.candidate-*"},
		{name: "config.generated.json", want: "config.generated.candidate-*.json"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := candidateConfigPattern(tt.name); got != tt.want {
				t.Fatalf("candidateConfigPattern(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestRuntimeBackendApplyConfigReloadFailureRollsBack(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	livePath := filepath.Join(dir, "config.json")
	liveConfig := []byte("{\"log\":{\"loglevel\":\"warning\"}}\n")
	if err := os.WriteFile(livePath, liveConfig, 0o644); err != nil {
		t.Fatalf("write live config: %v", err)
	}

	tester := &recordingConfigTester{}
	controller := &scriptedController{
		reloadErrs: []error{
			errors.New("reload failed"),
			nil,
		},
	}
	runtimeBackend := NewRuntimeBackend(livePath, controller)
	runtimeBackend.tester = tester

	err := runtimeBackend.ApplyConfig(context.Background(), testConfigRequest())
	if err == nil {
		t.Fatal("expected reload failure")
	}
	if !strings.Contains(err.Error(), "reload xray service") {
		t.Fatalf("unexpected error: %v", err)
	}
	if controller.reloadCalls != 2 {
		t.Fatalf("expected two reload attempts, got %d", controller.reloadCalls)
	}

	got, err := os.ReadFile(livePath)
	if err != nil {
		t.Fatalf("read live config: %v", err)
	}
	if !jsonEqual(t, got, liveConfig) {
		t.Fatalf("expected rollback to restore live config\nwant:\n%s\ngot:\n%s", liveConfig, got)
	}

	backup, err := os.ReadFile(runtimeBackend.backupPath)
	if err != nil {
		t.Fatalf("read backup config: %v", err)
	}
	if !jsonEqual(t, backup, liveConfig) {
		t.Fatalf("unexpected backup config\nwant:\n%s\ngot:\n%s", liveConfig, backup)
	}
}

func TestRuntimeBackendApplyConfigRollbackReloadFailureIncludesRollbackError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	livePath := filepath.Join(dir, "config.json")
	liveConfig := []byte("{\"log\":{\"loglevel\":\"warning\"}}\n")
	if err := os.WriteFile(livePath, liveConfig, 0o644); err != nil {
		t.Fatalf("write live config: %v", err)
	}

	controller := &scriptedController{
		reloadErrs: []error{
			errors.New("reload failed"),
			errors.New("rollback reload failed"),
		},
	}
	runtimeBackend := NewRuntimeBackend(livePath, controller)
	runtimeBackend.tester = &recordingConfigTester{}

	err := runtimeBackend.ApplyConfig(context.Background(), testConfigRequest())
	if err == nil {
		t.Fatal("expected rollback reload failure")
	}
	if !strings.Contains(err.Error(), "rollback xray service reload") {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := os.ReadFile(livePath)
	if err != nil {
		t.Fatalf("read live config: %v", err)
	}
	if !jsonEqual(t, got, liveConfig) {
		t.Fatalf("expected rollback to restore live config\nwant:\n%s\ngot:\n%s", liveConfig, got)
	}
}

func TestRuntimeBackendApplyConfigSuccessUpdatesLastKnownGood(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	livePath := filepath.Join(dir, "config.json")

	controller := &scriptedController{}
	runtimeBackend := NewRuntimeBackend(livePath, controller)
	runtimeBackend.tester = &recordingConfigTester{}

	if err := runtimeBackend.ApplyConfig(context.Background(), testConfigRequest()); err != nil {
		t.Fatalf("apply config: %v", err)
	}
	if controller.reloadCalls != 1 {
		t.Fatalf("expected one reload, got %d", controller.reloadCalls)
	}

	liveConfig, err := os.ReadFile(livePath)
	if err != nil {
		t.Fatalf("read live config: %v", err)
	}
	backup, err := os.ReadFile(runtimeBackend.backupPath)
	if err != nil {
		t.Fatalf("read backup config: %v", err)
	}
	if string(backup) != string(liveConfig) {
		t.Fatalf("expected backup to track last known good config\nwant:\n%s\ngot:\n%s", liveConfig, backup)
	}
}

func TestRuntimeBackendApplyConfigReloadFailureWithoutRollbackRemovesBrokenConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	livePath := filepath.Join(dir, "config.json")
	controller := &scriptedController{
		reloadErrs: []error{
			errors.New("reload failed"),
			nil,
		},
	}
	runtimeBackend := NewRuntimeBackend(livePath, controller)
	runtimeBackend.tester = &recordingConfigTester{}

	err := runtimeBackend.ApplyConfig(context.Background(), testConfigRequest())
	if err == nil {
		t.Fatal("expected reload failure")
	}

	if _, statErr := os.Stat(livePath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected failed config to be removed, got %v", statErr)
	}
	if _, statErr := os.Stat(runtimeBackend.backupPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected no backup file, got %v", statErr)
	}
}

func testConfigRequest() backend.ConfigRequest {
	return backend.ConfigRequest{
		Mode: domain.SelectionModeManual,
		Nodes: []domain.Node{
			{
				ID:       "node-1",
				Name:     "Edge",
				Protocol: domain.ProtocolVLESS,
				Address:  "203.0.113.10",
				Port:     443,
				UUID:     "11111111-1111-1111-1111-111111111111",
			},
		},
		SelectedNodeID: "node-1",
		LogLevel:       "warning",
		SOCKSPort:      10808,
		HTTPPort:       10809,
	}
}

type recordingConfigTester struct {
	configPaths []string
	err         error
}

func (t *recordingConfigTester) Test(_ context.Context, configPath string) error {
	t.configPaths = append(t.configPaths, configPath)
	return t.err
}

type scriptedController struct {
	reloadErrs  []error
	reloadCalls int
}

func (c *scriptedController) Start(context.Context) error { return nil }
func (c *scriptedController) Stop(context.Context) error  { return nil }

func (c *scriptedController) Reload(context.Context) error {
	call := c.reloadCalls
	c.reloadCalls++
	if call < len(c.reloadErrs) && c.reloadErrs[call] != nil {
		return c.reloadErrs[call]
	}
	return nil
}

func (c *scriptedController) Status(context.Context) (backend.RuntimeStatus, error) {
	return backend.RuntimeStatus{Running: true, ServiceState: "running"}, nil
}

func jsonEqual(t *testing.T, got, want []byte) bool {
	t.Helper()

	var gotValue any
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatalf("unmarshal got json: %v", err)
	}

	var wantValue any
	if err := json.Unmarshal(want, &wantValue); err != nil {
		t.Fatalf("unmarshal want json: %v", err)
	}

	gotNormalized, err := json.Marshal(gotValue)
	if err != nil {
		t.Fatalf("marshal got json: %v", err)
	}

	wantNormalized, err := json.Marshal(wantValue)
	if err != nil {
		t.Fatalf("marshal want json: %v", err)
	}

	return string(gotNormalized) == string(wantNormalized)
}
