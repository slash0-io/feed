package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/slash0-io/feed/feedschema"
)

func buildOpts(t *testing.T, fixtures, out, previous string) buildOptions {
	t.Helper()
	return buildOptions{
		SourcesPath:    "../sources.yaml",
		OutDir:         out,
		FixturesDir:    fixtures,
		Previous:       previous,
		MaxRemovedFrac: 0.5,
		MinGuardCount:  8,
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// copyFixtures clones the fixtures directory so a test can mutate one
// upstream body without touching the archive.
func copyFixtures(t *testing.T) string {
	t.Helper()
	dst := t.TempDir()
	entries, err := os.ReadDir(fixturesDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		b := mustRead(t, filepath.Join(fixturesDir, e.Name()))
		if err := os.WriteFile(filepath.Join(dst, e.Name()), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dst
}

func mutateStripeAPI(t *testing.T, fixtures string, replace bool) {
	t.Helper()
	p := filepath.Join(fixtures, "stripe-api.data")
	var d map[string][]string
	if err := json.Unmarshal(mustRead(t, p), &d); err != nil {
		t.Fatal(err)
	}
	if replace {
		d["API"] = []string{"198.51.100.7"} // wipes out the previous set → guardrail territory
	} else {
		d["API"] = append(d["API"], "198.51.100.7")
	}
	b, _ := json.Marshal(d)
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIncrementalIdempotent(t *testing.T) {
	dist1 := t.TempDir()
	s1, err := runBuild(buildOpts(t, fixturesDir, dist1, ""))
	if err != nil {
		t.Fatal(err)
	}
	if s1.Fresh == 0 || s1.Failed > 0 {
		t.Fatalf("fresh build: %+v", s1)
	}

	dist2 := t.TempDir()
	s2, err := runBuild(buildOpts(t, fixturesDir, dist2, filepath.Join(dist1, "v1")))
	if err != nil {
		t.Fatal(err)
	}
	if s2.BuildChanged || s2.Changed != 0 || s2.Unchanged != s1.Fresh {
		t.Fatalf("rebuild against previous: %+v (want all %d unchanged)", s2, s1.Fresh)
	}
	for _, rel := range []string{"v1/index.json", "v1/services/stripe.json", "v1/services/azure.json"} {
		if string(mustRead(t, filepath.Join(dist1, rel))) != string(mustRead(t, filepath.Join(dist2, rel))) {
			t.Errorf("%s differs between idempotent builds", rel)
		}
	}
	if got := string(mustRead(t, filepath.Join(dist2, "BUILD_CHANGED"))); got != "false\n" {
		t.Errorf("BUILD_CHANGED = %q, want false", got)
	}
}

func TestIncrementalChangePublishesAndLogs(t *testing.T) {
	dist1 := t.TempDir()
	if _, err := runBuild(buildOpts(t, fixturesDir, dist1, "")); err != nil {
		t.Fatal(err)
	}

	fixtures := copyFixtures(t)
	mutateStripeAPI(t, fixtures, false)

	dist2 := t.TempDir()
	s, err := runBuild(buildOpts(t, fixtures, dist2, filepath.Join(dist1, "v1")))
	if err != nil {
		t.Fatal(err)
	}
	if !s.BuildChanged || s.Changed != 1 || s.Quarantined != 0 {
		t.Fatalf("stats = %+v, want exactly one changed service", s)
	}
	// Untouched services republish byte-for-byte.
	if string(mustRead(t, filepath.Join(dist1, "v1/services/github.json"))) !=
		string(mustRead(t, filepath.Join(dist2, "v1/services/github.json"))) {
		t.Error("github.json should be byte-identical")
	}
	// The changed service gets fresh tokens and the new range.
	var stripe feedschema.Service
	if err := json.Unmarshal(mustRead(t, filepath.Join(dist2, "v1/services/stripe.json")), &stripe); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range stripe.Purposes["api"].IPv4 {
		if r == "198.51.100.7/32" {
			found = true
		}
	}
	if !found {
		t.Error("stripe api missing the added range")
	}
	// Changelog records the diff.
	var cl []feedschema.ChangelogEntry
	if err := json.Unmarshal(mustRead(t, filepath.Join(dist2, "v1/changelog.json")), &cl); err != nil {
		t.Fatal(err)
	}
	if len(cl) != 1 || len(cl[0].Changes) != 1 ||
		cl[0].Changes[0].Slug != "stripe" || cl[0].Changes[0].Purpose != "api" ||
		cl[0].Changes[0].Added != 1 || cl[0].Changes[0].Removed != 0 {
		t.Errorf("changelog = %+v, want one stripe/api +1/-0 entry", cl)
	}
}

func TestFailedFetchKeepsPreviousVersion(t *testing.T) {
	dist1 := t.TempDir()
	if _, err := runBuild(buildOpts(t, fixturesDir, dist1, "")); err != nil {
		t.Fatal(err)
	}

	fixtures := copyFixtures(t)
	for _, f := range []string{"stripe-api.data", "stripe-webhooks.data", "stripe-terminal.data"} {
		if err := os.Remove(filepath.Join(fixtures, f)); err != nil {
			t.Fatal(err)
		}
	}

	dist2 := t.TempDir()
	s, err := runBuild(buildOpts(t, fixtures, dist2, filepath.Join(dist1, "v1")))
	if err != nil {
		t.Fatal(err)
	}
	if s.Failed != 1 {
		t.Fatalf("stats = %+v, want one failed service", s)
	}
	// The service must remain in the published catalog, byte-for-byte.
	if string(mustRead(t, filepath.Join(dist1, "v1/services/stripe.json"))) !=
		string(mustRead(t, filepath.Join(dist2, "v1/services/stripe.json"))) {
		t.Error("failed stripe must keep serving the previously published version")
	}
	if s.BuildChanged {
		t.Error("BUILD_CHANGED should be false when a failed service falls back to previous")
	}
}

func TestQuarantineKeepsPreviousVersion(t *testing.T) {
	dist1 := t.TempDir()
	if _, err := runBuild(buildOpts(t, fixturesDir, dist1, "")); err != nil {
		t.Fatal(err)
	}

	fixtures := copyFixtures(t)
	mutateStripeAPI(t, fixtures, true) // replace the whole set → mass removal

	dist2 := t.TempDir()
	s, err := runBuild(buildOpts(t, fixtures, dist2, filepath.Join(dist1, "v1")))
	if err != nil {
		t.Fatal(err)
	}
	if s.Quarantined != 1 || s.Changed != 0 {
		t.Fatalf("stats = %+v, want exactly one quarantined service", s)
	}
	// The previously published document keeps serving, byte-for-byte.
	if string(mustRead(t, filepath.Join(dist1, "v1/services/stripe.json"))) !=
		string(mustRead(t, filepath.Join(dist2, "v1/services/stripe.json"))) {
		t.Error("quarantined stripe.json must equal the previously published version")
	}
	// Nothing else changed, so the publish as a whole is a no-op.
	if s.BuildChanged {
		t.Error("BUILD_CHANGED should be false when the only change is quarantined")
	}
}
