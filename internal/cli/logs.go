package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

const (
	logreadPath     = "/sbin/logread"
	xrayLogPath     = "/var/log/xray.log"
	defaultLogLimit = 200
)

var runLogread = defaultRunLogread
var readLogTail = defaultReadLogTail

type logsSnapshot struct {
	Available bool     `json:"available"`
	Source    string   `json:"source"`
	Error     string   `json:"error,omitempty"`
	RouteFlux []string `json:"routeflux"`
	Xray      []string `json:"xray"`
	System    []string `json:"system"`
}

func newLogsCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "logs",
		Short: "Show recent RouteFlux, Xray, and system logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			snapshot := buildLogsSnapshot(cmd.Context(), defaultLogLimit)

			if opts.jsonOutput {
				return printOutput(cmd, true, snapshot, "")
			}

			return printOutput(cmd, false, nil, renderLogsText(snapshot))
		},
	}
}

func buildLogsSnapshot(ctx context.Context, limit int) logsSnapshot {
	snapshot := logsSnapshot{
		Available: false,
		Source:    logreadPath,
		RouteFlux: []string{},
		Xray:      []string{},
		System:    []string{},
	}
	var errorsList []string
	var sources []string

	output, err := runLogread(ctx)
	if err == nil {
		lines := splitLogLines(output)
		snapshot.RouteFlux = lastN(filterLogLines(lines, []string{"routeflux["}), limit)
		snapshot.Xray = lastN(filterLogLines(lines, []string{"xray["}), limit)
		snapshot.System = lastN(lines, limit)
		sources = append(sources, logreadPath)
	} else {
		errorsList = append(errorsList, err.Error())
	}

	xrayLines, err := readLogTail(xrayLogPath, limit)
	switch {
	case err == nil:
		snapshot.Xray = xrayLines
		sources = append(sources, xrayLogPath)
	case errors.Is(err, os.ErrNotExist):
	default:
		errorsList = append(errorsList, fmt.Sprintf("read %s: %v", xrayLogPath, err))
	}

	if len(sources) > 0 {
		snapshot.Available = true
		snapshot.Source = strings.Join(sources, " + ")
	}
	if len(errorsList) > 0 {
		snapshot.Error = strings.Join(errorsList, "; ")
	}

	return snapshot
}

func defaultRunLogread(ctx context.Context) ([]byte, error) {
	cmd := exec.CommandContext(ctx, logreadPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("run %s: %w: %s", logreadPath, err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

func defaultReadLogTail(path string, limit int) ([]string, error) {
	if limit <= 0 {
		return []string{}, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() == 0 {
		return []string{}, nil
	}

	const chunkSize int64 = 4096
	var chunks [][]byte
	var newlineCount int

	for offset := info.Size(); offset > 0 && newlineCount <= limit; {
		readSize := chunkSize
		if offset < readSize {
			readSize = offset
		}

		start := offset - readSize
		chunk := make([]byte, readSize)
		n, readErr := file.ReadAt(chunk, start)
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return nil, readErr
		}

		chunk = chunk[:n]
		newlineCount += bytes.Count(chunk, []byte{'\n'})
		chunks = append(chunks, chunk)
		offset = start
	}

	totalSize := 0
	for _, chunk := range chunks {
		totalSize += len(chunk)
	}

	combined := make([]byte, 0, totalSize)
	for i := len(chunks) - 1; i >= 0; i-- {
		combined = append(combined, chunks[i]...)
	}

	return lastN(splitLogLines(combined), limit), nil
}

func splitLogLines(output []byte) []string {
	rawLines := bytes.Split(output, []byte{'\n'})
	lines := make([]string, 0, len(rawLines))
	for _, raw := range rawLines {
		line := strings.TrimSpace(string(raw))
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func filterLogLines(lines []string, patterns []string) []string {
	if len(patterns) == 0 {
		return append([]string(nil), lines...)
	}

	loweredPatterns := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(strings.ToLower(pattern))
		if pattern != "" {
			loweredPatterns = append(loweredPatterns, pattern)
		}
	}

	if len(loweredPatterns) == 0 {
		return append([]string(nil), lines...)
	}

	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		lowered := strings.ToLower(line)
		for _, pattern := range loweredPatterns {
			if strings.Contains(lowered, pattern) {
				filtered = append(filtered, line)
				break
			}
		}
	}

	return filtered
}

func lastN(lines []string, limit int) []string {
	if limit <= 0 || len(lines) <= limit {
		return append([]string(nil), lines...)
	}
	return append([]string(nil), lines[len(lines)-limit:]...)
}

func renderLogsText(snapshot logsSnapshot) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("available=%t\n", snapshot.Available))
	builder.WriteString(fmt.Sprintf("source=%s\n", snapshot.Source))
	if snapshot.Error != "" {
		builder.WriteString(fmt.Sprintf("error=%s\n", snapshot.Error))
	}

	builder.WriteString("\n[RouteFlux]\n")
	builder.WriteString(strings.Join(snapshot.RouteFlux, "\n"))
	builder.WriteString("\n\n[Xray]\n")
	builder.WriteString(strings.Join(snapshot.Xray, "\n"))
	builder.WriteString("\n\n[System]\n")
	builder.WriteString(strings.Join(snapshot.System, "\n"))

	return strings.TrimSpace(builder.String())
}
