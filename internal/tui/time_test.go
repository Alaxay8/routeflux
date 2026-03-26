package tui

import (
	"testing"
	"time"
)

func TestFormatLocalTimestampUsesProcessLocalZone(t *testing.T) {
	t.Setenv("TZ", "GMT-3")

	value := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)

	if got := formatLocalTimestamp(value); got != "2026-03-26 15:00:00 GMT" {
		t.Fatalf("unexpected local timestamp: %q", got)
	}
}
