package store

import "testing"

func TestDecodeProxySettings(t *testing.T) {
	for _, tc := range []struct {
		input       string
		lan         bool
		socks, http int
	}{
		{`{"schema_version":10}`, false, 10808, 10809},
		{`{"proxy":{"allow_lan":true,"socks_port":20808,"http_port":20809}}`, true, 20808, 20809},
		{`{"proxy":{"allow_lan":true}}`, true, 10808, 10809},
	} {
		got, err := decodeSettings([]byte(tc.input), "settings.json")
		if err != nil {
			t.Fatal(err)
		}
		if got.Proxy.AllowLAN != tc.lan || got.Proxy.SOCKSPort != tc.socks || got.Proxy.HTTPPort != tc.http {
			t.Fatalf("proxy: %+v", got.Proxy)
		}
	}
}

func TestProxySettingsSurviveStoreReload(t *testing.T) {
	root := t.TempDir()
	fileStore := NewFileStore(root)
	settings, err := fileStore.LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.Proxy.AllowLAN = true
	settings.Proxy.HTTPPort = 20809
	if err = fileStore.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	restored, err := NewFileStore(root).LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if restored.Proxy != settings.Proxy {
		t.Fatalf("reload: %+v", restored.Proxy)
	}
}
