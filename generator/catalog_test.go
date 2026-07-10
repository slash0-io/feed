package main

import (
	"os"
	"strings"
	"testing"
)

// TestCatalogInSync enforces that the committed CATALOG.md matches what the
// registry generates — the catalog must never drift from sources.yaml.
func TestCatalogInSync(t *testing.T) {
	reg, err := LoadRegistry("../sources.yaml")
	if err != nil {
		t.Fatal(err)
	}
	want := renderCatalogMarkdown(reg)
	got, err := os.ReadFile("../CATALOG.md")
	if err != nil {
		t.Fatalf("CATALOG.md missing — regenerate: go run ./generator -catalog CATALOG.md (%v)", err)
	}
	if string(got) != want {
		t.Fatal("CATALOG.md is stale — regenerate: go run ./generator -catalog CATALOG.md")
	}
}

func TestCatalogContent(t *testing.T) {
	reg, err := LoadRegistry("../sources.yaml")
	if err != nil {
		t.Fatal(err)
	}
	md := renderCatalogMarkdown(reg)
	for _, want := range []string{"| `stripe` |", "| `aws` |", "`api` (egress)", "`hooks` (ingress)", "## Services that do NOT publish"} {
		if !strings.Contains(md, want) {
			t.Errorf("catalog markdown missing %q", want)
		}
	}
	// Every service in the registry must appear in exactly one category section.
	for _, s := range reg.Services {
		if !strings.Contains(md, "| `"+s.Slug+"` |") {
			t.Errorf("service %s missing from catalog (unknown category %q?)", s.Slug, s.Category)
		}
	}
	html := renderCatalogHTML(reg)
	for _, want := range []string{"<code>stripe</code>", "v1/services/stripe.json", "Doesn't publish"} {
		if !strings.Contains(html, want) {
			t.Errorf("catalog html missing %q", want)
		}
	}
}
