package displaytime

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var posixTZPattern = regexp.MustCompile(`^(?:<([^>]+)>|([A-Za-z+-]+))([+-]\d{1,2})(?::(\d{2}))?(?::(\d{2}))?`)

// Format converts the input timestamp into the best available local display timezone.
func Format(value time.Time, layout string) string {
	return value.In(location()).Format(layout)
}

// FormatString parses an RFC3339 timestamp and formats it in the local display timezone.
func FormatString(value string, layout string) string {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return value
	}

	return Format(parsed, layout)
}

func location() *time.Location {
	for _, candidate := range []string{
		strings.TrimSpace(os.Getenv("TZ")),
		strings.TrimSpace(readFile("/etc/TZ")),
	} {
		if loc := parseLocation(candidate); loc != nil {
			return loc
		}
	}

	if time.Local != nil {
		return time.Local
	}
	return time.UTC
}

func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func parseLocation(raw string) *time.Location {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	if loc, err := time.LoadLocation(raw); err == nil {
		return loc
	}

	upper := strings.ToUpper(raw)
	switch upper {
	case "UTC", "GMT":
		return time.UTC
	}

	matches := posixTZPattern.FindStringSubmatch(raw)
	if len(matches) == 0 {
		return nil
	}

	name := firstNonEmpty(matches[1], matches[2], raw)
	offsetHours, err := strconv.Atoi(matches[3])
	if err != nil {
		return nil
	}

	offsetMinutes := 0
	if matches[4] != "" {
		offsetMinutes, err = strconv.Atoi(matches[4])
		if err != nil {
			return nil
		}
	}

	offsetSeconds := 0
	if matches[5] != "" {
		offsetSeconds, err = strconv.Atoi(matches[5])
		if err != nil {
			return nil
		}
	}

	total := offsetHours*3600 + sign(offsetHours)*offsetMinutes*60 + sign(offsetHours)*offsetSeconds
	return time.FixedZone(name, -total)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func sign(value int) int {
	if value < 0 {
		return -1
	}
	return 1
}
