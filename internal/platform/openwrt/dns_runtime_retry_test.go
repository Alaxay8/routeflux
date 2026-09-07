package openwrt

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Alaxay8/routeflux/internal/domain"
)

func dnsRetryFixture(t *testing.T) (DNSRuntimeManager, string, string) {
	t.Helper()
	dir := t.TempDir()
	proc := filepath.Join(dir, "proc")
	pid := filepath.Join(proc, "1")
	conf := filepath.Join(dir, "conf")
	resolv := filepath.Join(dir, "resolv")
	for _, p := range []string{pid, conf} {
		if err := os.MkdirAll(p, 0755); err != nil {
			t.Fatal(err)
		}
	}
	for p, data := range map[string]string{filepath.Join(pid, "comm"): "dnsmasq\n", filepath.Join(pid, "cmdline"): "dnsmasq\x00--conf-dir=" + conf + "\x00--resolv-file=" + resolv + "\x00", resolv: "nameserver 1.1.1.1\n"} {
		if err := os.WriteFile(p, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	log := filepath.Join(dir, "calls")
	fail := filepath.Join(dir, "fail")
	script := writeExecutable(t, filepath.Join(dir, "service"), "#!/bin/sh\necho restart >> '"+log+"'\nif [ -f '"+fail+"' ]; then exit 1; fi\n")
	return DNSRuntimeManager{ProcRoot: proc, DNSMasqSnippetPath: filepath.Join(conf, "routeflux-dns.conf"), DNSMasqServicePath: script}, log, fail
}

func TestDNSRuntimeRetriesIncompleteChanges(t *testing.T) {
	for _, operation := range []string{"apply", "disable"} {
		for _, failure := range []string{"restart", "canceled"} {
			t.Run(operation+"/"+failure, func(t *testing.T) {
				m, log, fail := dnsRetryFixture(t)
				if operation == "disable" {
					if err := os.WriteFile(m.DNSMasqSnippetPath, []byte("server=127.0.0.1#1053\n"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				invoke := func(ctx context.Context, manager DNSRuntimeManager) error {
					if operation == "disable" {
						return manager.Disable(ctx)
					}
					return manager.Apply(ctx, domain.DNSSettings{Mode: domain.DNSModeRemote}, "127.0.0.1", 1053)
				}
				ctx := context.Background()
				if failure == "restart" {
					if err := os.WriteFile(fail, nil, 0600); err != nil {
						t.Fatal(err)
					}
				} else {
					var cancel context.CancelFunc
					ctx, cancel = context.WithCancel(ctx)
					cancel()
				}
				if err := invoke(ctx, m); err == nil {
					t.Fatal("expected initial failure")
				}
				marker := dnsRestartMarkerPath(m.DNSMasqSnippetPath)
				info, err := os.Stat(marker)
				if err != nil {
					t.Fatal(err)
				}
				if !strings.HasPrefix(filepath.Base(marker), ".") || info.Mode().Perm() != 0600 {
					t.Fatal("marker must be hidden and private")
				}
				if err := os.Remove(fail); err != nil && !os.IsNotExist(err) {
					t.Fatal(err)
				}
				fresh := DNSRuntimeManager{ProcRoot: m.ProcRoot, DNSMasqSnippetPath: m.DNSMasqSnippetPath, DNSMasqServicePath: m.DNSMasqServicePath}
				if err := invoke(context.Background(), fresh); err != nil {
					t.Fatal(err)
				}
				calls, err := os.ReadFile(log)
				if err != nil {
					t.Fatal(err)
				}
				want := 1
				if failure == "restart" {
					want = 2
				}
				if strings.Count(string(calls), "restart") != want {
					t.Fatalf("restart was not retried: %s", calls)
				}
				if err := invoke(context.Background(), fresh); err != nil {
					t.Fatal(err)
				}
				after, err := os.ReadFile(log)
				if err != nil {
					t.Fatal(err)
				}
				if string(after) != string(calls) {
					t.Fatal("successful duplicate restarted DNS")
				}
			})
		}
	}
}

func TestDNSRuntimeDisableReportsUnknownPath(t *testing.T) {
	m := DNSRuntimeManager{ProcRoot: t.TempDir()}
	if err := m.Disable(context.Background()); err == nil {
		t.Fatal("reported success without locating DNS configuration")
	}
}

func TestDNSRuntimeMarkerErrorsPreventFalseSuccess(t *testing.T) {
	for _, operation := range []string{"apply", "disable"} {
		t.Run(operation, func(t *testing.T) {
			m, log, _ := dnsRetryFixture(t)
			original := "server=127.0.0.1#1053\n"
			if err := os.WriteFile(m.DNSMasqSnippetPath, []byte(original), 0600); err != nil {
				t.Fatal(err)
			}
			marker := dnsRestartMarkerPath(m.DNSMasqSnippetPath)
			if err := os.Mkdir(marker, 0700); err != nil {
				t.Fatal(err)
			}
			var err error
			if operation == "apply" {
				err = m.Apply(context.Background(), domain.DNSSettings{Mode: domain.DNSModeRemote}, "127.0.0.1", 2053)
			} else {
				err = m.Disable(context.Background())
			}
			if err == nil || !strings.Contains(err.Error(), "marker") {
				t.Fatalf("expected marker error, got %v", err)
			}
			data, err := os.ReadFile(m.DNSMasqSnippetPath)
			if err != nil || string(data) != original {
				t.Fatalf("snippet changed despite marker failure: %s, %v", data, err)
			}
			if _, err := os.Stat(log); !os.IsNotExist(err) {
				t.Fatalf("unexpected restart: %v", err)
			}
		})
	}
}

func TestDNSRuntimeReportsMarkerRemovalFailure(t *testing.T) {
	m, _, _ := dnsRetryFixture(t)
	marker := dnsRestartMarkerPath(m.DNSMasqSnippetPath)
	m.DNSMasqServicePath = writeExecutable(t, m.DNSMasqServicePath, "#!/bin/sh\nset -e\nrm '"+marker+"'\nmkdir '"+marker+"'\ntouch '"+marker+"/blocked'\n")
	err := m.Apply(context.Background(), domain.DNSSettings{Mode: domain.DNSModeRemote}, "127.0.0.1", 1053)
	if err == nil || !strings.Contains(err.Error(), "remove DNS restart marker") {
		t.Fatalf("expected cleanup error, got %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("incomplete application marker lost")
	}
}

func TestDNSRestartMarkerWriteFailure(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(parent, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := markDNSRestartPending(filepath.Join(parent, "snippet")); err == nil {
		t.Fatal("expected marker write failure")
	}
}
