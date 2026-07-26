package main

import (
	"os"
	"strings"
	"testing"
)

const sourcesDocPath = "../research/SOURCES.md"

// TestSourcesDocInSync enforces that the generated tables in the research
// document match sources.yaml. The hand-maintained version of these tables
// drifted for weeks without anyone noticing, which is the whole reason they
// are generated now.
func TestSourcesDocInSync(t *testing.T) {
	reg, err := LoadRegistry("../sources.yaml")
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(sourcesDocPath)
	if err != nil {
		t.Fatalf("SOURCES.md missing, regenerate: go run ./generator -sources-doc research/SOURCES.md (%v)", err)
	}
	want, err := rewriteSourcesDoc(string(body), reg)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != want {
		t.Fatal("SOURCES.md is stale, regenerate: go run ./generator -sources-doc research/SOURCES.md")
	}
}

// TestSourcesDocClaimsAreCited is the point of the exercise: no row may state
// an advance notice or a vendor signal without linking the page that says so.
func TestSourcesDocClaimsAreCited(t *testing.T) {
	reg, err := LoadRegistry("../sources.yaml")
	if err != nil {
		t.Fatal(err)
	}
	table, err := renderSourcesCatalogTable(reg)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(table, "\n") {
		if !strings.HasPrefix(line, "| [") {
			continue
		}
		cells := strings.Split(line, "|")
		// cells: "", service, document, detection, notice, signal, direction, ""
		if len(cells) != 8 {
			t.Fatalf("unexpected row shape: %q", line)
		}
		for i, name := range map[int]string{4: "notice", 5: "signal"} {
			cell := strings.TrimSpace(cells[i])
			if cell == "none documented" || cell == "none" {
				continue
			}
			if !strings.Contains(cell, "](http") {
				t.Errorf("%s claim without an evidence link: %q", name, line)
			}
		}
	}
}

// TestSourcesDocRejectsUncitedClaim proves the build actually fails rather
// than quietly emitting a bare claim, since that failure mode is what the
// evidence requirement exists to prevent.
func TestSourcesDocRejectsUncitedClaim(t *testing.T) {
	reg, err := LoadRegistry("../sources.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for i := range reg.Services {
		if reg.Services[i].Notice != "" {
			reg.Services[i].NoticeEvidence = ""
			if _, err := renderSourcesCatalogTable(reg); err == nil {
				t.Fatal("stripping notice_evidence should fail the render")
			}
			return
		}
	}
	t.Skip("no service documents a notice period")
}

func TestSourcesDocRejectsEmDash(t *testing.T) {
	reg, err := LoadRegistry("../sources.yaml")
	if err != nil {
		t.Fatal(err)
	}
	body := sourcesCatalogStart + "\n" + sourcesCatalogEnd + "\n" +
		sourcesNonPubStart + "\n" + sourcesNonPubEnd + "\nprose with an em dash — here\n"
	if _, err := rewriteSourcesDoc(body, reg); err == nil {
		t.Fatal("an em dash in the document should fail the render")
	}
}
