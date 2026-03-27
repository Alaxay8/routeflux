package displaytime

import (
	"testing"
	"time"
)

func TestFormatUsesTZEnvironmentPOSIXOffset(t *testing.T) {
	t.Setenv("TZ", "GMT-3")

	value := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)

	if got := Format(value, time.RFC3339); got != "2026-03-26T15:00:00+03:00" {
		t.Fatalf("unexpected formatted time: %q", got)
	}
}

func TestFormatStringUsesTZEnvironmentPOSIXOffset(t *testing.T) {
	t.Setenv("TZ", "GMT-3")

	if got := FormatString("2026-03-26T12:00:00Z", time.RFC3339); got != "2026-03-26T15:00:00+03:00" {
		t.Fatalf("unexpected formatted string: %q", got)
	}
}
