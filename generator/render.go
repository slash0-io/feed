package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"
)

// Rendering vendor pages with a headless browser.
//
// A few vendors publish their ranges only into a client-rendered page. Duo's
// Salesforce-community article and UptimeRobot's locations page both return
// HTML containing zero addresses to a plain fetch, and both render completely
// under a browser. Setting `render: chrome` on an endpoint opts it into this
// path.
//
// This runs only against the network. Fixture builds read the stored rendered
// DOM, so `go test` and the offline CI build never need a browser installed.
//
// A render that silently produces an error page is not a special case worth
// detecting here: it yields no addresses, and the zero-ranges guardrail in
// buildService already refuses to publish a purpose that parsed to nothing.

const (
	renderTimeout     = 120 * time.Second
	virtualTimeBudget = "25000" // ms of simulated time before the DOM is dumped

	// renderUserAgent keeps the browser token these pages need in order to
	// render at all, and appends the same identifier the HTTP path sends so a
	// vendor reading their logs can tell who we are and where to complain.
	renderUserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) " +
		"Chrome/126.0.0.0 Safari/537.36 slash0-feed-generator/0.1 (+https://github.com/slash0-io/feed)"
)

// chromeCandidates are searched in order when SLASH0_CHROME is unset. The
// Linux names come first because that is what CI runs.
var chromeCandidates = []string{
	"google-chrome", "google-chrome-stable", "chromium", "chromium-browser",
	"/usr/bin/google-chrome", "/usr/bin/chromium",
	"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
	"/Applications/Chromium.app/Contents/MacOS/Chromium",
}

// chromePath resolves the browser binary, preferring an explicit override.
func chromePath() (string, error) {
	if p := os.Getenv("SLASH0_CHROME"); p != "" {
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("SLASH0_CHROME=%s is not usable: %w", p, err)
		}
		return p, nil
	}
	for _, c := range chromeCandidates {
		if p, err := exec.LookPath(c); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("no Chrome or Chromium found on %s; set SLASH0_CHROME to the binary "+
		"(only endpoints with `render: chrome` need it)", runtime.GOOS)
}

// renderChrome loads url in a headless browser and returns the resulting DOM.
func renderChrome(url string) ([]byte, error) {
	bin, err := chromePath()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), renderTimeout)
	defer cancel()

	args := []string{
		"--headless=new",
		"--disable-gpu",
		"--no-sandbox",            // CI runs as root in a container
		"--disable-dev-shm-usage", // /dev/shm is small on CI runners
		"--no-first-run",
		"--disable-extensions",
		"--user-agent=" + renderUserAgent,
		"--virtual-time-budget=" + virtualTimeBudget,
		"--dump-dom",
		url,
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("render %s: timed out after %s", url, renderTimeout)
		}
		return nil, fmt.Errorf("render %s: %w (%s)", url, err, firstLine(stderr.String()))
	}
	if stdout.Len() == 0 {
		return nil, fmt.Errorf("render %s: browser returned an empty document", url)
	}
	return stdout.Bytes(), nil
}

func firstLine(s string) string {
	if i := bytes.IndexByte([]byte(s), '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
