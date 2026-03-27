package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Alaxay8/routeflux/internal/buildinfo"
)

func TestVersionCommandOutputsBuildInfo(t *testing.T) {
	restoreBuildInfo(t, "1.2.3", "abc1234", "2026-03-27T18:10:00Z")

	cmd := newRootCmd()
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute version command: %v", err)
	}

	output := stdout.String()
	wants := []string{
		"RouteFlux 1.2.3",
		"Commit: abc1234",
		"Built: 2026-03-27T18:10:00Z",
	}
	for _, want := range wants {
		if !strings.Contains(output, want) {
			t.Fatalf("version output missing %q\n%s", want, output)
		}
	}
}

func TestVersionFlagOutputsBuildInfo(t *testing.T) {
	restoreBuildInfo(t, "1.2.3", "def5678", "2026-03-27T18:11:00Z")

	cmd := newRootCmd()
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"--version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute version flag: %v", err)
	}

	output := stdout.String()
	wants := []string{
		"RouteFlux 1.2.3",
		"Commit: def5678",
		"Built: 2026-03-27T18:11:00Z",
	}
	for _, want := range wants {
		if !strings.Contains(output, want) {
			t.Fatalf("version flag output missing %q\n%s", want, output)
		}
	}
}

func TestVersionCommandJSONOutputsBuildInfo(t *testing.T) {
	restoreBuildInfo(t, "1.2.3", "feedbee", "2026-03-27T18:12:00Z")

	cmd := newRootCmd()
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"--json", "version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute version command json: %v", err)
	}

	var info buildinfo.Info
	if err := json.Unmarshal(stdout.Bytes(), &info); err != nil {
		t.Fatalf("unmarshal version json: %v\n%s", err, stdout.String())
	}

	if info.Version != "1.2.3" {
		t.Fatalf("unexpected version: %s", info.Version)
	}
	if info.Commit != "feedbee" {
		t.Fatalf("unexpected commit: %s", info.Commit)
	}
	if info.BuildDate != "2026-03-27T18:12:00Z" {
		t.Fatalf("unexpected build date: %s", info.BuildDate)
	}
}

func restoreBuildInfo(t *testing.T, version string, commit string, buildDate string) {
	t.Helper()

	prevVersion := buildinfo.Version
	prevCommit := buildinfo.Commit
	prevBuildDate := buildinfo.BuildDate
	buildinfo.Version = version
	buildinfo.Commit = commit
	buildinfo.BuildDate = buildDate
	t.Cleanup(func() {
		buildinfo.Version = prevVersion
		buildinfo.Commit = prevCommit
		buildinfo.BuildDate = prevBuildDate
	})
}
