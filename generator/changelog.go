package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"

	"github.com/slash0-io/feed/feedschema"
)

// changelogCap bounds the published changelog.
const changelogCap = 500

// loadChangelogFloor reads the checked-in changelog floor. dist/ is not
// tracked and Pages deployments are the only copy of the published
// changelog, so the floor file is the durable lower bound on history: it is
// merged into every publish, which both survives and repairs a publish that
// lost entries.
func loadChangelogFloor(path string) ([]feedschema.ChangelogEntry, error) {
	if path == "" {
		return nil, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("changelog floor: %w", err)
	}
	var out []feedschema.ChangelogEntry
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("changelog floor %s: %w", path, err)
	}
	return out, nil
}

// mergeChangelog unions the published changelog with the floor, newest
// first. Published entries win on sync-token collision.
func mergeChangelog(live, floor []feedschema.ChangelogEntry) []feedschema.ChangelogEntry {
	if len(floor) == 0 {
		return live
	}
	seen := make(map[string]bool, len(live))
	for _, e := range live {
		seen[e.SyncToken] = true
	}
	out := append([]feedschema.ChangelogEntry{}, live...)
	for _, e := range floor {
		if !seen[e.SyncToken] {
			out = append(out, e)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return changelogLess(out[j], out[i]) })
	if len(out) > changelogCap {
		out = out[:changelogCap]
	}
	return out
}

// changelogLess orders entries oldest-first by sync token (unix seconds),
// falling back to the timestamp string.
func changelogLess(a, b feedschema.ChangelogEntry) bool {
	at, aerr := strconv.ParseInt(a.SyncToken, 10, 64)
	bt, berr := strconv.ParseInt(b.SyncToken, 10, 64)
	if aerr == nil && berr == nil {
		return at < bt
	}
	return a.PublishedAt < b.PublishedAt
}

func changelogEqual(a, b []feedschema.ChangelogEntry) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return bytes.Equal(ab, bb)
}
