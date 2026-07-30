package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	want := map[string]int{"duo": 22, "uptimerobot": 205}
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
