package openwrt_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
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

	luciClient, sessionCookies, err := h.openLuCISession(smokeCtx)
	if err != nil {
		return err
	}
	if err := h.waitForLuCIRoutePage(smokeCtx, luciClient); err != nil {
		return err
	}

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

	var snapshot luciPageSnapshot
	if err := chromedp.Run(smokeCtx,
		runtime.Enable(),
		network.Enable(),
		setBrowserCookies(h.luciURL("/cgi-bin/luci/"), sessionCookies),
		chromedp.Navigate(h.luciURL("/cgi-bin/luci/admin/services/routeflux/subscriptions")),
		chromedp.WaitReady(`body`, chromedp.ByQuery),
		waitForRenderedSubscriptionsPage(expectedTexts, &snapshot),
	); err != nil {
		return fmt.Errorf("browser smoke subscriptions page: %w", err)
	}

	runtimeMu.Lock()
	defer runtimeMu.Unlock()
	if len(runtimeErrors) > 0 {
		return fmt.Errorf("browser runtime exception: %s", runtimeErrors[0])
	}

	return nil
}

func (h *openWRTHarness) AssertLuCIRoutingPage(ctx context.Context, expectedTexts ...string) error {
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

	luciClient, sessionCookies, err := h.openLuCISession(smokeCtx)
	if err != nil {
		return err
	}
	if err := h.waitForLuCIRoutePage(smokeCtx, luciClient); err != nil {
		return err
	}

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

	var snapshot luciPageSnapshot
	if err := chromedp.Run(smokeCtx,
		runtime.Enable(),
		network.Enable(),
		setBrowserCookies(h.luciURL("/cgi-bin/luci/"), sessionCookies),
		chromedp.Navigate(h.luciURL("/cgi-bin/luci/admin/services/routeflux/firewall")),
		chromedp.WaitReady(`body`, chromedp.ByQuery),
		waitForRenderedRoutingPage(expectedTexts, &snapshot),
	); err != nil {
		return fmt.Errorf("browser smoke routing page: %w", err)
	}

	runtimeMu.Lock()
	defer runtimeMu.Unlock()
	if len(runtimeErrors) > 0 {
		return fmt.Errorf("browser runtime exception: %s", runtimeErrors[0])
	}

	return nil
}

type luciPageSnapshot struct {
	BodyHTML       string `json:"bodyHTML"`
	BodyText       string `json:"bodyText"`
	HasAddSource   bool   `json:"hasAddSource"`
	HasLoginForm   bool   `json:"hasLoginForm"`
	HasRoutingRoot bool   `json:"hasRoutingRoot"`
	HasRoot        bool   `json:"hasRoot"`
	PageError      string `json:"pageError"`
	Title          string `json:"title"`
	URL            string `json:"url"`
}

func (h *openWRTHarness) openLuCISession(ctx context.Context) (*http.Client, []*http.Cookie, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, nil, fmt.Errorf("create LuCI cookie jar: %w", err)
	}

	client := &http.Client{
		Jar:     jar,
		Timeout: 15 * time.Second,
	}

	rpcErr := h.authenticateLuCIViaRPC(ctx, client)
	if cookies := jar.Cookies(h.luciBaseURL()); len(cookies) > 0 {
		return client, cookies, nil
	}

	formErr := h.authenticateLuCIViaForm(ctx, client)
	cookies := jar.Cookies(h.luciBaseURL())
	if len(cookies) == 0 {
		return nil, nil, fmt.Errorf("authenticate LuCI session: rpc auth: %v; form auth: %v", rpcErr, formErr)
	}

	return client, cookies, nil
}

func (h *openWRTHarness) authenticateLuCIViaRPC(ctx context.Context, client *http.Client) error {
	body, err := json.Marshal(map[string]any{
		"id":     1,
		"method": "login",
		"params": []string{"root", luciTestPassword},
	})
	if err != nil {
		return fmt.Errorf("marshal LuCI RPC auth request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.luciURL("/cgi-bin/luci/rpc/auth"), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build LuCI RPC auth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("POST LuCI RPC auth: %w", err)
	}
	defer resp.Body.Close()

	payload, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if readErr != nil {
		return fmt.Errorf("read LuCI RPC auth response: %w", readErr)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("LuCI RPC auth status %s: %s", resp.Status, summarizeForError(string(payload), 240))
	}

	return nil
}

func (h *openWRTHarness) authenticateLuCIViaForm(ctx context.Context, client *http.Client) error {
	loginURL := h.luciURL("/cgi-bin/luci/")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, loginURL, nil)
	if err != nil {
		return fmt.Errorf("build LuCI login GET request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("GET LuCI login page: %w", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	form := url.Values{
		"luci_username": {"root"},
		"luci_password": {luciTestPassword},
	}
	req, err = http.NewRequestWithContext(ctx, http.MethodPost, loginURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build LuCI login POST request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err = client.Do(req)
	if err != nil {
		return fmt.Errorf("POST LuCI login form: %w", err)
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if readErr != nil {
		return fmt.Errorf("read LuCI login form response: %w", readErr)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("LuCI login form status %s: %s", resp.Status, summarizeForError(string(body), 240))
	}

	return nil
}

func (h *openWRTHarness) waitForLuCIRoutePage(ctx context.Context, client *http.Client) error {
	pageURL := h.luciURL("/cgi-bin/luci/admin/services/routeflux/subscriptions")
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
		if err != nil {
			return fmt.Errorf("build LuCI subscriptions request: %w", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return fmt.Errorf("wait for LuCI subscriptions route: %w", ctx.Err())
			}
			time.Sleep(500 * time.Millisecond)
			continue
		}

		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		resp.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read LuCI subscriptions route response: %w", readErr)
		}

		page := string(body)
		if resp.StatusCode == http.StatusOK &&
			!strings.Contains(page, `id="luci_password"`) &&
			(strings.Contains(page, "routeflux/subscriptions") || strings.Contains(page, "RouteFlux")) {
			return nil
		}

		if ctx.Err() != nil {
			return fmt.Errorf("wait for LuCI subscriptions route: %w; last status=%s body=%q", ctx.Err(), resp.Status, summarizeForError(page, 320))
		}

		time.Sleep(500 * time.Millisecond)
	}
}

func waitForRenderedSubscriptionsPage(expectedTexts []string, snapshot *luciPageSnapshot) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		var last luciPageSnapshot
		var lastErr error
		var readySince time.Time
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			current := luciPageSnapshot{}
			if err := captureLuCIPageSnapshot(&current).Do(ctx); err != nil {
				lastErr = err
			} else {
				last = current
				lastErr = nil
				if current.HasLoginForm {
					return fmt.Errorf("browser returned to LuCI login page: url=%s title=%q", current.URL, current.Title)
				}
				if current.PageError != "" {
					return fmt.Errorf("subscriptions page error: %s", summarizeForError(current.PageError, 240))
				}
				if current.HasRoot && current.HasAddSource && containsAllText(current.BodyText, expectedTexts) {
					if snapshot != nil {
						*snapshot = current
					}
					return nil
				}
				if current.HasRoot && current.HasAddSource {
					if readySince.IsZero() {
						readySince = time.Now()
					}
					if time.Since(readySince) >= 10*time.Second {
						return fmt.Errorf("subscriptions page rendered but missing expected text %v; url=%s title=%q body=%q", missingText(current.BodyText, expectedTexts), current.URL, current.Title, summarizeForError(current.BodyText, 320))
					}
				} else {
					readySince = time.Time{}
				}
			}

			select {
			case <-ctx.Done():
				parts := []string{fmt.Sprintf("wait for rendered subscriptions page: %v", ctx.Err())}
				if lastErr != nil {
					parts = append(parts, fmt.Sprintf("last snapshot error=%v", lastErr))
				}
				if last.URL != "" {
					parts = append(parts, fmt.Sprintf("last url=%s", last.URL))
				}
				if last.Title != "" {
					parts = append(parts, fmt.Sprintf("last title=%q", last.Title))
				}
				parts = append(parts,
					fmt.Sprintf("hasRoot=%t", last.HasRoot),
					fmt.Sprintf("hasAddSource=%t", last.HasAddSource),
					fmt.Sprintf("body=%q", summarizeForError(last.BodyText, 320)),
				)
				return fmt.Errorf("%s", strings.Join(parts, "; "))
			case <-ticker.C:
			}
		}
	})
}

func waitForRenderedRoutingPage(expectedTexts []string, snapshot *luciPageSnapshot) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		var last luciPageSnapshot
		var lastErr error
		var readySince time.Time
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			current := luciPageSnapshot{}
			if err := captureLuCIPageSnapshot(&current).Do(ctx); err != nil {
				lastErr = err
			} else {
				last = current
				lastErr = nil
				if current.HasLoginForm {
					return fmt.Errorf("browser returned to LuCI login page: url=%s title=%q", current.URL, current.Title)
				}
				if current.PageError != "" {
					return fmt.Errorf("routing page error: %s", summarizeForError(current.PageError, 240))
				}
				if current.HasRoutingRoot && containsAllText(current.BodyText, expectedTexts) {
					if snapshot != nil {
						*snapshot = current
					}
					return nil
				}
				if current.HasRoutingRoot {
					if readySince.IsZero() {
						readySince = time.Now()
					}
					if time.Since(readySince) >= 10*time.Second {
						return fmt.Errorf("routing page rendered but missing expected text %v; url=%s title=%q body=%q", missingText(current.BodyText, expectedTexts), current.URL, current.Title, summarizeForError(current.BodyText, 320))
					}
				} else {
					readySince = time.Time{}
				}
			}

			select {
			case <-ctx.Done():
				parts := []string{fmt.Sprintf("wait for rendered routing page: %v", ctx.Err())}
				if lastErr != nil {
					parts = append(parts, fmt.Sprintf("last snapshot error=%v", lastErr))
				}
				if last.URL != "" {
					parts = append(parts, fmt.Sprintf("last url=%s", last.URL))
				}
				if last.Title != "" {
					parts = append(parts, fmt.Sprintf("last title=%q", last.Title))
				}
				parts = append(parts,
					fmt.Sprintf("hasRoutingRoot=%t", last.HasRoutingRoot),
					fmt.Sprintf("body=%q", summarizeForError(last.BodyText, 320)),
				)
				return fmt.Errorf("%s", strings.Join(parts, "; "))
			case <-ticker.C:
			}
		}
	})
}

func (h *openWRTHarness) luciURL(path string) string {
	return fmt.Sprintf("http://127.0.0.1:%d%s", h.httpPort, path)
}

func (h *openWRTHarness) luciBaseURL() *url.URL {
	base, err := url.Parse(h.luciURL("/cgi-bin/luci/"))
	if err != nil {
		panic(err)
	}
	return base
}

func setBrowserCookies(pageURL string, cookies []*http.Cookie) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		params, err := browserCookieParams(pageURL, cookies)
		if err != nil {
			return err
		}
		return network.SetCookies(params).Do(ctx)
	})
}

func browserCookieParams(pageURL string, cookies []*http.Cookie) ([]*network.CookieParam, error) {
	params := make([]*network.CookieParam, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie == nil || strings.TrimSpace(cookie.Name) == "" {
			continue
		}

		param := &network.CookieParam{
			Name:     cookie.Name,
			Value:    cookie.Value,
			URL:      pageURL,
			Path:     cookie.Path,
			Secure:   cookie.Secure,
			HTTPOnly: cookie.HttpOnly,
		}
		if param.Path == "" {
			param.Path = "/"
		}
		if cookie.Domain != "" {
			param.Domain = strings.TrimPrefix(cookie.Domain, ".")
		}

		params = append(params, param)
	}

	if len(params) == 0 {
		return nil, fmt.Errorf("set browser cookies: no cookies to apply")
	}

	return params, nil
}

func captureLuCIPageSnapshot(snapshot *luciPageSnapshot) chromedp.Action {
	return chromedp.Evaluate(`(() => {
		const body = document.body;
		const pageError = document.querySelector('.routeflux-page-banner-warning');

		return {
			bodyHTML: body ? body.innerHTML : '',
			bodyText: body ? body.innerText : '',
			hasAddSource: !!document.querySelector('#routeflux-add-source'),
			hasLoginForm: !!document.querySelector('#luci_password'),
			hasRoutingRoot: !!document.querySelector('#routeflux-routing-root'),
			hasRoot: !!document.querySelector('#routeflux-subscriptions-root'),
			pageError: pageError ? pageError.innerText.trim() : '',
			title: document.title || '',
			url: window.location.href
		};
	})()`, snapshot)
}

func containsAllText(body string, expectedTexts []string) bool {
	for _, expected := range expectedTexts {
		if !strings.Contains(body, expected) {
			return false
		}
	}
	return true
}

func missingText(body string, expectedTexts []string) []string {
	missing := make([]string, 0, len(expectedTexts))
	for _, expected := range expectedTexts {
		if !strings.Contains(body, expected) {
			missing = append(missing, expected)
		}
	}
	return missing
}

func summarizeForError(value string, limit int) string {
	text := strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if limit <= 0 || len(text) <= limit {
		return text
	}
	return text[:limit] + "..."
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
