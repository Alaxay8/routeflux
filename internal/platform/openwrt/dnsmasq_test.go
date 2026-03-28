package openwrt

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Alaxay8/routeflux/internal/domain"
)

func TestFirewallManagerValidateRejectsDNSMasqWithoutNFTSet(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dnsmasqPath := writeExecutable(t, filepath.Join(dir, "dnsmasq"), "#!/bin/sh\nif [ \"$1\" = \"--test\" ]; then\n  echo 'dnsmasq: recompile with HAVE_NFTSET defined to enable nftset directives' >&2\n  exit 1\nfi\necho '--nftset supported but disabled'\n")

	manager := FirewallManager{
		DNSMasqPath:        dnsmasqPath,
		DNSMasqSnippetPath: filepath.Join(dir, "routeflux.conf"),
	}

	err := manager.Validate(context.Background(), domain.FirewallSettings{
		Enabled:        true,
		TargetServices: []string{"youtube"},
	})
	if err == nil {
		t.Fatal("expected dnsmasq validation to fail")
	}
	if !strings.Contains(err.Error(), "dnsmasq-full") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFirewallManagerApplyWritesDNSMasqSnippetAndReloads(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.log")
	nftPath := writeExecutable(t, filepath.Join(dir, "nft"), "#!/bin/sh\nprintf 'nft %s\\n' \"$*\" >> \""+logPath+"\"\nexit 0\n")
	dnsmasqPath := writeExecutable(t, filepath.Join(dir, "dnsmasq"), "#!/bin/sh\nif [ \"$1\" = \"--test\" ]; then\n  exit 0\nfi\necho 'Dnsmasq test binary'\n")
	servicePath := writeExecutable(t, filepath.Join(dir, "dnsmasq-service"), "#!/bin/sh\nprintf '%s\\n' \"$1\" >> \""+logPath+"\"\nexit 0\n")
	snippetPath := filepath.Join(dir, "routeflux.conf")

	manager := FirewallManager{
		NFTPath:            nftPath,
		RulesPath:          filepath.Join(dir, "routeflux-firewall.nft"),
		DNSMasqPath:        dnsmasqPath,
		DNSMasqServicePath: servicePath,
		DNSMasqSnippetPath: snippetPath,
	}

	settings := domain.FirewallSettings{
		Enabled:         true,
		TransparentPort: 12345,
		TargetServices:  []string{"youtube", "telegram"},
		TargetCIDRs:     []string{"1.1.1.1"},
		TargetDomains:   []string{"youtube.com"},
		TargetServiceCatalog: map[string]domain.FirewallTargetDefinition{
			"openai": {
				Domains: []string{"openai.com", "chatgpt.com"},
			},
		},
		BlockQUIC: true,
	}

	if err := manager.Apply(context.Background(), settings); err != nil {
		t.Fatalf("apply firewall: %v", err)
	}

	snippet, err := os.ReadFile(snippetPath)
	if err != nil {
		t.Fatalf("read dnsmasq snippet: %v", err)
	}

	for _, want := range []string{
		"nftset=/youtube.com/4#inet#routeflux#target_v4",
		"nftset=/youtu.be/4#inet#routeflux#target_v4",
		"nftset=/youtube-nocookie.com/4#inet#routeflux#target_v4",
		"nftset=/youtubei.googleapis.com/4#inet#routeflux#target_v4",
		"nftset=/youtube.googleapis.com/4#inet#routeflux#target_v4",
		"nftset=/googlevideo.com/4#inet#routeflux#target_v4",
		"nftset=/telegram.org/4#inet#routeflux#target_v4",
		"nftset=/t.me/4#inet#routeflux#target_v4",
		"nftset=/ytimg.com/4#inet#routeflux#target_v4",
		"nftset=/ggpht.com/4#inet#routeflux#target_v4",
	} {
		if !strings.Contains(string(snippet), want) {
			t.Fatalf("snippet missing %q\n%s", want, snippet)
		}
	}

	calls, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read call log: %v", err)
	}

	if !strings.Contains(string(calls), "reload") {
		t.Fatalf("expected dnsmasq reload, got %q", calls)
	}
}

func TestFirewallManagerApplyWritesDNSMasqSnippetForCustomServices(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.log")
	nftPath := writeExecutable(t, filepath.Join(dir, "nft"), "#!/bin/sh\nprintf 'nft %s\\n' \"$*\" >> \""+logPath+"\"\nexit 0\n")
	dnsmasqPath := writeExecutable(t, filepath.Join(dir, "dnsmasq"), "#!/bin/sh\nif [ \"$1\" = \"--test\" ]; then\n  exit 0\nfi\necho 'Dnsmasq test binary'\n")
	servicePath := writeExecutable(t, filepath.Join(dir, "dnsmasq-service"), "#!/bin/sh\nprintf '%s\\n' \"$1\" >> \""+logPath+"\"\nexit 0\n")
	snippetPath := filepath.Join(dir, "routeflux.conf")

	manager := FirewallManager{
		NFTPath:            nftPath,
		RulesPath:          filepath.Join(dir, "routeflux-firewall.nft"),
		DNSMasqPath:        dnsmasqPath,
		DNSMasqServicePath: servicePath,
		DNSMasqSnippetPath: snippetPath,
	}

	settings := domain.FirewallSettings{
		Enabled:         true,
		TransparentPort: 12345,
		TargetServices:  []string{"openai"},
		TargetServiceCatalog: map[string]domain.FirewallTargetDefinition{
			"openai": {
				Domains: []string{"openai.com", "chatgpt.com"},
			},
		},
	}

	if err := manager.Apply(context.Background(), settings); err != nil {
		t.Fatalf("apply firewall: %v", err)
	}

	snippet, err := os.ReadFile(snippetPath)
	if err != nil {
		t.Fatalf("read dnsmasq snippet: %v", err)
	}

	for _, want := range []string{
		"nftset=/openai.com/4#inet#routeflux#target_v4",
		"nftset=/chatgpt.com/4#inet#routeflux#target_v4",
	} {
		if !strings.Contains(string(snippet), want) {
			t.Fatalf("snippet missing %q\n%s", want, snippet)
		}
	}
}

func TestFirewallManagerDisableRemovesDNSMasqSnippet(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	nftPath := writeExecutable(t, filepath.Join(dir, "nft"), "#!/bin/sh\nexit 0\n")
	servicePath := writeExecutable(t, filepath.Join(dir, "dnsmasq-service"), "#!/bin/sh\nexit 0\n")
	snippetPath := filepath.Join(dir, "routeflux.conf")
	if err := os.WriteFile(snippetPath, []byte("nftset=/youtube.com/4#inet#routeflux#target_v4\n"), 0o644); err != nil {
		t.Fatalf("write snippet: %v", err)
	}

	manager := FirewallManager{
		NFTPath:            nftPath,
		RulesPath:          filepath.Join(dir, "routeflux-firewall.nft"),
		DNSMasqServicePath: servicePath,
		DNSMasqSnippetPath: snippetPath,
	}

	if err := manager.Disable(context.Background()); err != nil {
		t.Fatalf("disable firewall: %v", err)
	}

	if _, err := os.Stat(snippetPath); !os.IsNotExist(err) {
		t.Fatalf("expected snippet to be removed, stat err=%v", err)
	}
}

func writeExecutable(t *testing.T, path, contents string) string {
	t.Helper()

	if err := os.WriteFile(path, []byte(contents), 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}

	return path
}
