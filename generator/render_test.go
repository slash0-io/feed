package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeRegistry builds a minimal one-endpoint registry so the render config
// can be validated without a browser or a network.
func writeRegistry(t *testing.T, endpoint string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sources.yaml")
	body := "schema_version: 1\nservices:\n  - slug: x\n    name: X\n    category: cloud\n" +
		"    classification: dedicated\n    provenance: https://example.com\n" +
		"    endpoints:\n" + endpoint
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRenderEngineMustBeKnown(t *testing.T) {
	path := writeRegistry(t, "      - id: a\n        url: https://example.com\n"+
		"        format: html-cidr-extract\n        render: firefox\n")
	_, err := LoadRegistry(path)
	if err == nil {
		t.Fatal("an unknown render engine should fail to load")
	}
	if !strings.Contains(err.Error(), "render engine") {
		t.Errorf("error should name the offending field, got %v", err)
	}
}

// Chrome takes its headers from the page load rather than from our process, so
// accepting both would leave a pinned API version looking applied when it is
// not. Rejecting the combination keeps that from shipping silently.
func TestRenderRejectsHeaders(t *testing.T) {
	path := writeRegistry(t, "      - id: a\n        url: https://example.com\n"+
		"        format: html-cidr-extract\n        render: chrome\n"+
		"        headers: {revision: \"2026-01-01\"}\n")
	if _, err := LoadRegistry(path); err == nil {
		t.Fatal("render plus headers should fail to load")
	}
}

func TestRenderChromeIsOptIn(t *testing.T) {
	path := writeRegistry(t, "      - id: a\n        url: https://example.com\n"+
		"        format: html-cidr-extract\n")
	r, err := LoadRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Services[0].Endpoints[0].Render; got != "" {
		t.Errorf("render should default to empty, got %q", got)
	}
}

// A misconfigured browser path must say so plainly rather than surfacing later
// as a vendor that mysteriously stopped parsing.
func TestChromePathReportsBadOverride(t *testing.T) {
	t.Setenv("SLASH0_CHROME", filepath.Join(t.TempDir(), "definitely-not-here"))
	_, err := chromePath()
	if err == nil {
		t.Fatal("a nonexistent SLASH0_CHROME should be an error")
	}
	if !strings.Contains(err.Error(), "SLASH0_CHROME") {
		t.Errorf("error should name the override, got %v", err)
	}
}

// The rendered vendors must stay parseable from their stored DOM, so the
// offline build and `go test` never need a browser installed.
func TestRenderedFixturesParseWithoutBrowser(t *testing.T) {
	t.Setenv("SLASH0_CHROME", filepath.Join(t.TempDir(), "definitely-not-here"))
	reg, err := LoadRegistry("../sources.yaml")
	if err != nil {
		t.Fatal(err)
	}
	// Counts exclude aggregate purposes, which are the union of the others and
	// would double-count a split service against itself.
	want := map[string]int{"duo": 29, "uptimerobot": 205}
	f := NewFetcher("../testdata/fixtures")
	for _, svc := range reg.Services {
		n, ok := want[svc.Slug]
		if !ok {
			continue
		}
		if svc.Endpoints[0].Render == "" {
			t.Errorf("%s is expected to be a rendered source", svc.Slug)
		}
		doc, err := buildService(svc, f, "2026-07-30T00:00:00Z", "1")
		if err != nil {
			t.Fatalf("%s: %v", svc.Slug, err)
		}
		total := 0
		for _, p := range doc.Purposes {
			if p.Aggregate {
				continue
			}
			total += len(p.IPv4) + len(p.IPv6)
		}
		if total != n {
			t.Errorf("%s = %d ranges, want %d", svc.Slug, total, n)
		}
		delete(want, svc.Slug)
	}
	for slug := range want {
		t.Errorf("%s is missing from sources.yaml", slug)
	}
}

// A vendor that splits IPv4 and IPv6 across two endpoints must end up with one
// purpose carrying both. Assigning rather than merging would silently publish
// only whichever endpoint happened to be listed last.
func TestPurposeMergesAcrossEndpoints(t *testing.T) {
	reg, err := LoadRegistry("../sources.yaml")
	if err != nil {
		t.Fatal(err)
	}
	f := NewFetcher("../testdata/fixtures")
	for _, svc := range reg.Services {
		if svc.Slug != "checkly" {
			continue
		}
		if len(svc.Endpoints) < 2 {
			t.Fatalf("checkly should feed one purpose from two endpoints, has %d", len(svc.Endpoints))
		}
		doc, err := buildService(svc, f, "2026-08-12T00:00:00Z", "1")
		if err != nil {
			t.Fatal(err)
		}
		p, ok := doc.Purposes["probes"]
		if !ok {
			t.Fatal("checkly has no probes purpose")
		}
		if len(p.IPv4) == 0 || len(p.IPv6) == 0 {
			t.Errorf("probes should carry both families, got v4=%d v6=%d", len(p.IPv4), len(p.IPv6))
		}
		return
	}
	t.Fatal("checkly is missing from sources.yaml")
}

// mergeRanges must never widen the covered set, the same invariant the
// single-document aggregation holds.
func TestMergeRangesIsLossless(t *testing.T) {
	// Public space on purpose: normalize drops RFC1918, so private test
	// fixtures would pass vacuously by producing nothing at all.
	got := mergeRanges([]string{"203.0.113.0/25"}, []string{"203.0.113.128/25"})
	if len(got) != 1 || got[0] != "203.0.113.0/24" {
		t.Errorf("adjacent halves should merge to the parent, got %v", got)
	}
	got = mergeRanges([]string{"198.51.100.0/24"}, []string{"203.0.113.0/24"})
	if len(got) != 2 {
		t.Errorf("disjoint ranges must both survive, got %v", got)
	}
	if out := mergeRanges(nil, []string{"198.51.100.0/24"}); len(out) != 1 {
		t.Errorf("empty left side should pass the right through, got %v", out)
	}
}

func fastRenderRetries(t *testing.T) {
	t.Helper()
	old := renderRetryDelays
	renderRetryDelays = []time.Duration{0}
	t.Cleanup(func() { renderRetryDelays = old })
}

// fakeBrowser writes a script standing in for Chrome. It fails its first
// failCount invocations and succeeds afterwards, counting through a file so the
// count survives across processes.
func fakeBrowser(t *testing.T, failCount int, output string) string {
	t.Helper()
	dir := t.TempDir()
	counter := filepath.Join(dir, "n")
	script := filepath.Join(dir, "browser")
	body := fmt.Sprintf(`#!/bin/sh
n=0
[ -f %[1]q ] && n=$(cat %[1]q)
n=$((n+1))
printf '%%s' "$n" > %[1]q
if [ "$n" -le %[2]d ]; then
  echo "simulated crash" >&2
  exit 1
fi
printf '%%s' %[3]q
`, counter, failCount, output)
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SLASH0_CHROME", script)
	return counter
}

// A hung or crashed render is transient. It cost five publish runs in a week,
// every one of them help.make.com timing out on a page that renders in about 25
// seconds, while the HTTP path beside it had retried transient failures all
// along.
func TestRenderChromeRetriesATransientFailure(t *testing.T) {
	fastRenderRetries(t)
	counter := fakeBrowser(t, 1, "<html><body>198.51.100.0/24</body></html>")

	body, err := renderChrome("https://example.test/ips")
	if err != nil {
		t.Fatalf("render failed despite a retry being available: %v", err)
	}
	if !strings.Contains(string(body), "198.51.100.0/24") {
		t.Errorf("got %q, want the rendered document", body)
	}
	if n, _ := os.ReadFile(counter); string(n) != "2" {
		t.Errorf("browser ran %s times, want 2 (one failure then one success)", n)
	}
}

// A browser that never succeeds still fails, and does so bounded rather than
// looping.
func TestRenderChromeGivesUpAfterItsRetries(t *testing.T) {
	fastRenderRetries(t)
	counter := fakeBrowser(t, 99, "")

	if _, err := renderChrome("https://example.test/ips"); err == nil {
		t.Fatal("a browser that always fails must fail the render")
	}
	want := fmt.Sprint(len(renderRetryDelays) + 1)
	if n, _ := os.ReadFile(counter); string(n) != want {
		t.Errorf("browser ran %s times, want %s", n, want)
	}
}

// An empty document is a failed render, not an empty page: publishing it would
// look like a vendor removing every range.
func TestRenderChromeRetriesAnEmptyDocument(t *testing.T) {
	fastRenderRetries(t)
	counter := fakeBrowser(t, 0, "")

	if _, err := renderChrome("https://example.test/ips"); err == nil {
		t.Fatal("an empty document must not be returned as a successful render")
	}
	want := fmt.Sprint(len(renderRetryDelays) + 1)
	if n, _ := os.ReadFile(counter); string(n) != want {
		t.Errorf("browser ran %s times, want %s", n, want)
	}
}
