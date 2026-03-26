package cli

import (
	"testing"
	"time"
)

func TestFormatLocalTimestampUsesProcessLocalZone(t *testing.T) {
	t.Setenv("TZ", "GMT-3")

	value := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)

	if got := formatLocalTimestamp(value); got != "2026-03-26T15:00:00+03:00" {
		t.Fatalf("unexpected local timestamp: %q", got)
	}
}
