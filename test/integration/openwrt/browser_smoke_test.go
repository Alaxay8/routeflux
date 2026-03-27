package openwrt_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

var browserEnvNames = []string{
	"ROUTEFLUX_OPENWRT_BROWSER_BIN",
	"CHROME_BIN",
	"CHROMIUM_BIN",
	"BROWSER",
}

var browserCandidates = []string{
	"headless_shell",
	"headless-shell",
	"chromium",
	"chromium-browser",
	"google-chrome",
	"google-chrome-stable",
	"google-chrome-beta",
	"google-chrome-unstable",
	"/usr/bin/google-chrome",
	"/usr/local/bin/chrome",
	"/snap/bin/chromium",
	"chrome",
}

func (h *openWRTHarness) AssertLuCISubscriptionsPage(ctx context.Context, expectedTexts ...string) error {
	browserPath, err := lookupBrowserBinary()
	if err != nil {
		return err
	}

	allocatorOptions := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(browserPath),
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
	)

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, allocatorOptions...)
	defer cancelAlloc()

	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()

	smokeCtx, cancelSmoke := context.WithTimeout(browserCtx, 90*time.Second)
	defer cancelSmoke()

	var runtimeErrors []string
	var runtimeMu sync.Mutex
	chromedp.ListenTarget(smokeCtx, func(ev any) {
		switch e := ev.(type) {
		case *runtime.EventExceptionThrown:
			details := ""
			if e.ExceptionDetails.Exception != nil {
				details = strings.TrimSpace(e.ExceptionDetails.Exception.Description)
			}
			if details == "" {
				details = strings.TrimSpace(e.ExceptionDetails.Text)
			}
			if details == "" {
				details = "unknown JavaScript exception"
			}
			runtimeMu.Lock()
			runtimeErrors = append(runtimeErrors, details)
			runtimeMu.Unlock()
		}
	})

	var pageText string
	if err := chromedp.Run(smokeCtx,
		runtime.Enable(),
		chromedp.Navigate(h.luciURL("/cgi-bin/luci/")),
		chromedp.WaitVisible(`#luci_username`, chromedp.ByID),
		chromedp.SetValue(`#luci_username`, "", chromedp.ByID),
		chromedp.SendKeys(`#luci_username`, "root", chromedp.ByID),
		chromedp.SendKeys(`#luci_password`, luciTestPassword, chromedp.ByID),
		chromedp.Click(`button.cbi-button-positive`, chromedp.ByQuery),
		chromedp.WaitVisible(`#modemenu`, chromedp.ByID),
		chromedp.Navigate(h.luciURL("/cgi-bin/luci/admin/services/routeflux/subscriptions")),
		chromedp.WaitVisible(`#routeflux-subscriptions-root`, chromedp.ByID),
		chromedp.WaitVisible(`#routeflux-add-source`, chromedp.ByID),
		chromedp.Text(`#routeflux-subscriptions-root`, &pageText, chromedp.ByID),
	); err != nil {
		return fmt.Errorf("browser smoke subscriptions page: %w", err)
	}

	runtimeMu.Lock()
	defer runtimeMu.Unlock()
	if len(runtimeErrors) > 0 {
		return fmt.Errorf("browser runtime exception: %s", runtimeErrors[0])
	}

	for _, expected := range expectedTexts {
		if strings.Contains(pageText, expected) {
			continue
		}
		return fmt.Errorf("subscriptions page missing %q in rendered text: %s", expected, pageText)
	}

	return nil
}

func (h *openWRTHarness) luciURL(path string) string {
	return fmt.Sprintf("http://127.0.0.1:%d%s", h.httpPort, path)
}

func lookupBrowserBinary() (string, error) {
	return resolveBrowserBinary(os.LookupEnv, exec.LookPath)
}

func resolveBrowserBinary(lookupEnv func(string) (string, bool), lookPath func(string) (string, error)) (string, error) {
	for _, envName := range browserEnvNames {
		if value, ok := lookupEnv(envName); ok {
			path := strings.TrimSpace(value)
			if path != "" {
				if strings.Contains(path, string(os.PathSeparator)) {
					return path, nil
				}
				resolved, err := lookPath(path)
				if err == nil {
					return resolved, nil
				}
				return path, nil
			}
		}
	}

	for _, candidate := range browserCandidates {
		path, err := lookPath(candidate)
		if err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("find browser binary: install chromium/google-chrome/headless-shell or set ROUTEFLUX_OPENWRT_BROWSER_BIN")
}
