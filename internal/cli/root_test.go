package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootHelpDoesNotListCompletionCommand(t *testing.T) {
	cmd := newRootCmd()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute root help: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "status") {
		t.Fatalf("root help missing known command:\n%s", output)
	}
	if strings.Contains(output, "completion") {
		t.Fatalf("root help unexpectedly lists completion command:\n%s", output)
	}
}

func TestRootCommandRejectsCompletionCommand(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"completion"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected unknown command error")
	}
	if !strings.Contains(err.Error(), `unknown command "completion"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
