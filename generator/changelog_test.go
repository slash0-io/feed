package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/slash0-io/feed/feedschema"
)

func TestChangelogFloorRestoresHistory(t *testing.T) {
	dist1 := t.TempDir()
	if _, err := runBuild(buildOpts(t, fixturesDir, dist1, "")); err != nil {
		t.Fatal(err)
	}

	floor := []feedschema.ChangelogEntry{{
		PublishedAt: "2026-07-20T00:00:00Z",
		SyncToken:   "1784505600",
		Changes:     []feedschema.ServiceChange{{Slug: "stripe", Purpose: "api", Added: 1}},
	}}
	floorPath := filepath.Join(t.TempDir(), "changelog-floor.json")
	b, _ := json.Marshal(floor)
	if err := os.WriteFile(floorPath, b, 0o644); err != nil {
		t.Fatal(err)
	}

	// The floor entry is missing from the published feed: restoring it is a
	// deployable change even though the catalog is untouched.
	dist2 := t.TempDir()
	opts := buildOpts(t, fixturesDir, dist2, filepath.Join(dist1, "v1"))
	opts.ChangelogFloor = floorPath
	s, err := runBuild(opts)
	if err != nil {
		t.Fatal(err)
	}
	if !s.BuildChanged {
		t.Fatalf("stats = %+v, want BuildChanged for restored changelog", s)
	}
	var cl []feedschema.ChangelogEntry
	if err := json.Unmarshal(mustRead(t, filepath.Join(dist2, "v1/changelog.json")), &cl); err != nil {
		t.Fatal(err)
	}
	if len(cl) != 1 || cl[0].SyncToken != "1784505600" {
		t.Fatalf("changelog = %+v, want exactly the floor entry", cl)
	}
	// A changelog restore is not a catalog publish: index bytes stay
	// verbatim.
	if string(mustRead(t, filepath.Join(dist1, "v1/index.json"))) !=
		string(mustRead(t, filepath.Join(dist2, "v1/index.json"))) {
		t.Error("index.json must not change on a changelog-only restore")
	}

	// Once the floor is published, the same floor is a no-op.
	dist3 := t.TempDir()
	opts = buildOpts(t, fixturesDir, dist3, filepath.Join(dist2, "v1"))
	opts.ChangelogFloor = floorPath
	if s, err = runBuild(opts); err != nil {
		t.Fatal(err)
	}
	if s.BuildChanged {
		t.Fatalf("stats = %+v, want byte-stable republish with published floor", s)
	}
}

func TestMergeChangelogOrdersAndDedupes(t *testing.T) {
	e := func(token string) feedschema.ChangelogEntry {
		return feedschema.ChangelogEntry{PublishedAt: "2026-07-2" + token[len(token)-1:] + "T00:00:00Z", SyncToken: token}
	}
	live := []feedschema.ChangelogEntry{e("300"), e("200")}
	floor := []feedschema.ChangelogEntry{e("400"), e("200"), e("100")}
	got := mergeChangelog(live, floor)
	want := []string{"400", "300", "200", "100"}
	if len(got) != len(want) {
		t.Fatalf("merged %d entries, want %d", len(got), len(want))
	}
	for i, token := range want {
		if got[i].SyncToken != token {
			t.Errorf("merged[%d] = %s, want %s", i, got[i].SyncToken, token)
		}
	}
}
