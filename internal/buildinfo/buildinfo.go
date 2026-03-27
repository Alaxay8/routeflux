package buildinfo

import (
	"fmt"
	"strings"
)

// These values are injected at build time. Safe defaults keep local `go test`
// and ad-hoc builds usable when ldflags are not provided.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
}

func Current() Info {
	return Info{
		Version:   normalize(Version, "dev"),
		Commit:    normalize(Commit, "unknown"),
		BuildDate: normalize(BuildDate, "unknown"),
	}
}

func Text() string {
	info := Current()
	return fmt.Sprintf("RouteFlux %s\nCommit: %s\nBuilt: %s", info.Version, info.Commit, info.BuildDate)
}

func normalize(value string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}
