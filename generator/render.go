package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
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

// renderRetryDelays paces one retry of a failed render. The HTTP path retries
// twice; this list is shorter on purpose, because every attempt here can burn
// the full renderTimeout. help.make.com renders in 20 to 26 seconds and the
// observed failure is an occasional hang to the deadline (5 publish runs of 40
// between 2026-09-06 and 2026-09-11, all of them that page, all of them
// timeouts). One retry turns a hang into a successful publish, while a page
// that is genuinely broken costs two attempts per run instead of three.
var renderRetryDelays = []time.Duration{3 * time.Second}

// renderChrome loads url in a headless browser and returns the resulting DOM,
// retrying a hung or crashed render. A missing browser is a configuration
// error rather than a transient one, so it fails immediately.
func renderChrome(url string) ([]byte, error) {
	bin, err := chromePath()
	if err != nil {
		return nil, err
	}
	for attempt := 0; ; attempt++ {
		body, retryable, err := renderOnce(bin, url)
		if err == nil {
			return body, nil
		}
		if !retryable || attempt >= len(renderRetryDelays) {
			return nil, err
		}
		log.Printf("render %s: %v — retrying", url, err)
		time.Sleep(renderRetryDelays[attempt])
	}
}

// renderOnce runs the browser once. Every failure it can produce is transient:
// a hang, a crashed browser, or an empty document. A page that has genuinely
// stopped carrying addresses still fails, through the zero-ranges guardrail in
// buildService, rather than through this function.
func renderOnce(bin, url string) (body []byte, retryable bool, err error) {
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
			return nil, true, fmt.Errorf("render %s: timed out after %s", url, renderTimeout)
		}
		return nil, true, fmt.Errorf("render %s: %w (%s)", url, err, firstLine(stderr.String()))
	}
	if stdout.Len() == 0 {
		return nil, true, fmt.Errorf("render %s: browser returned an empty document", url)
	}
	return stdout.Bytes(), false, nil
}

func firstLine(s string) string {
	if i := bytes.IndexByte([]byte(s), '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
