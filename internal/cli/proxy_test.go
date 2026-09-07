package cli

import (
	"bytes"
	"github.com/Alaxay8/routeflux/internal/app"
	"github.com/Alaxay8/routeflux/internal/domain"
	"testing"
)

func TestProxyCommandAtomicUpdate(t *testing.T) {
	store := &cliMemoryStore{settings: domain.DefaultSettings(), state: domain.DefaultRuntimeState()}
	for _, tc := range []struct {
		args []string
		fail bool
	}{
		{[]string{"set", "--allow-lan", "true", "--socks-port", "20808", "--http-port", "20809"}, false},
		{[]string{"set", "--socks-port", "20809"}, true},
		{[]string{"set", "--allow-lan", "typo"}, true},
		{[]string{"set", "--http-port", "0"}, true},
		{[]string{"set", "--http-port", "65536"}, true},
		{[]string{"get"}, false},
	} {
		cmd := newProxyCmd(&rootOptions{service: app.NewService(app.Dependencies{Store: store}), jsonOutput: true})
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))
		cmd.SetArgs(tc.args)
		err := cmd.Execute()
		if (err != nil) != tc.fail {
			t.Fatalf("%v: %v", tc.args, err)
		}
		if !store.settings.Proxy.AllowLAN || store.settings.Proxy.SOCKSPort != 20808 || store.settings.Proxy.HTTPPort != 20809 {
			t.Fatalf("settings changed unexpectedly: %+v", store.settings.Proxy)
		}
	}
}
