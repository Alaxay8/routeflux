package parser_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Alaxay8/routeflux/internal/parser"
)

func TestParseVLESSLink(t *testing.T) {
	t.Parallel()

	input := mustReadFixture(t, "vless", "subscription.txt")
	nodes, err := parser.ParseNodes(input, "Example Provider")
	if err != nil {
		t.Fatalf("parse nodes: %v", err)
	}

	assertGoldenNodes(t, nodes, "vless", "normalized.golden.json")
}

func TestParseVMessLink(t *testing.T) {
	t.Parallel()

	input := mustReadFixture(t, "vmess", "subscription.txt")
	nodes, err := parser.ParseNodes(input, "Example Provider")
	if err != nil {
		t.Fatalf("parse nodes: %v", err)
	}

	assertGoldenNodes(t, nodes, "vmess", "normalized.golden.json")
}

func TestParseMixedBase64Subscription(t *testing.T) {
	t.Parallel()

	input := mustReadFixture(t, "mixed", "subscription.b64")
	nodes, err := parser.ParseNodes(input, "Mixed Provider")
	if err != nil {
		t.Fatalf("parse nodes: %v", err)
	}

	assertGoldenNodes(t, nodes, "mixed", "normalized.golden.json")
}

func TestParseInvalidInput(t *testing.T) {
	t.Parallel()

	if _, err := parser.ParseNodes("not-a-subscription", "Broken"); err == nil {
		t.Fatal("expected invalid input to fail")
	}
}

func mustReadFixture(t *testing.T, parts ...string) string {
	t.Helper()

	path := filepath.Join(append([]string{"..", "..", "test", "fixtures"}, parts...)...)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}

	return string(data)
}

func assertGoldenNodes(t *testing.T, nodes any, fixtureDir, golden string) {
	t.Helper()

	rawGot, err := marshalCanonicalJSON(nodes)
	if err != nil {
		t.Fatalf("marshal nodes: %v", err)
	}

	got, err := normalizeJSONString(string(rawGot))
	if err != nil {
		t.Fatalf("normalize generated nodes: %v", err)
	}

	want, err := normalizeJSONString(mustReadFixture(t, fixtureDir, golden))
	if err != nil {
		t.Fatalf("normalize golden: %v", err)
	}

	if got != want {
		t.Fatalf("golden mismatch\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}

func normalizeJSONString(input string) (string, error) {
	var value any
	if err := json.Unmarshal([]byte(input), &value); err != nil {
		return "", err
	}

	data, err := marshalCanonicalJSON(value)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func marshalCanonicalJSON(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}

	return bytes.TrimSpace(buffer.Bytes()), nil
}
