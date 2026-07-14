package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/slash0-io/feed/feedschema"
)

// previousFeed is the currently published feed, loaded for incremental
// comparison. Nil means "no usable previous feed" and the build publishes
// fresh.
type previousFeed struct {
	index        feedschema.Index
	indexBytes   []byte
	services     map[string]*feedschema.Service
	serviceBytes map[string][]byte
	fingerprints map[string]string
	changelog    []feedschema.ChangelogEntry
}

// loadPrevious fetches base (an http(s):// URL or a local directory holding
// index.json). Any failure is logged and returns nil — an unreachable
// previous feed must degrade to a fresh publish, never fail the build.
func loadPrevious(base string) *previousFeed {
	if base == "" {
		return nil
	}
	get := previousGetter(base)

	ib, err := get("index.json")
	if err != nil {
		log.Printf("previous feed unavailable (%v) — publishing fresh", err)
		return nil
	}
	prev := &previousFeed{
		indexBytes:   ib,
		services:     map[string]*feedschema.Service{},
		serviceBytes: map[string][]byte{},
		fingerprints: map[string]string{},
	}
	if err := json.Unmarshal(ib, &prev.index); err != nil || prev.index.SchemaVersion != feedschema.SchemaVersion {
		log.Printf("previous index unusable (err=%v, schema=%d) — publishing fresh", err, prev.index.SchemaVersion)
		return nil
	}
	for _, entry := range prev.index.Services {
		b, err := get(entry.Path)
		if err != nil {
			log.Printf("previous %s unavailable (%v) — publishing fresh", entry.Slug, err)
			return nil
		}
		var svc feedschema.Service
		if err := json.Unmarshal(b, &svc); err != nil {
			log.Printf("previous %s unparseable (%v) — publishing fresh", entry.Slug, err)
			return nil
		}
		prev.services[entry.Slug] = &svc
		prev.serviceBytes[entry.Slug] = b
		prev.fingerprints[entry.Slug] = purposesFingerprint(svc.Purposes)
	}
	if cb, err := get("changelog.json"); err == nil {
		_ = json.Unmarshal(cb, &prev.changelog) // best-effort; absent on older feeds
	}
	return prev
}

func previousGetter(base string) func(rel string) ([]byte, error) {
	if strings.HasPrefix(base, "http://") || strings.HasPrefix(base, "https://") {
		client := &http.Client{Timeout: 30 * time.Second}
		base = strings.TrimSuffix(base, "/")
		return func(rel string) ([]byte, error) {
			resp, err := client.Get(base + "/" + rel)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("GET %s/%s: %s", base, rel, resp.Status)
			}
			return io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
		}
	}
	root := strings.TrimPrefix(base, "file://")
	return func(rel string) ([]byte, error) {
		return os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	}
}

// purposesFingerprint identifies a service's normalized content. JSON
// marshaling sorts map keys and normalize sorts the range lists, so equal
// content always produces equal fingerprints.
func purposesFingerprint(p map[string]feedschema.Purpose) string {
	b, err := json.Marshal(p)
	if err != nil {
		panic(err) // static types; cannot fail
	}
	return sha256hex(b)
}

// quarantineReason applies the mass-removal guardrail: a change that removes
// more than maxFrac of a purpose's previously published ranges (when the
// purpose had at least minCount ranges) must not auto-publish — a truncated
// or erroneous upstream body would otherwise propagate to every consumer.
func quarantineReason(prev, next map[string]feedschema.Purpose, maxFrac float64, minCount int) string {
	for key, pp := range prev {
		prevSet := rangeSet(pp)
		if len(prevSet) < minCount {
			continue
		}
		np, ok := next[key]
		if !ok {
			return fmt.Sprintf("purpose %q disappeared (%d ranges)", key, len(prevSet))
		}
		nextSet := rangeSet(np)
		removed := 0
		for r := range prevSet {
			if !nextSet[r] {
				removed++
			}
		}
		if frac := float64(removed) / float64(len(prevSet)); frac > maxFrac {
			return fmt.Sprintf("purpose %q: %d of %d ranges removed (%.0f%%)", key, removed, len(prevSet), frac*100)
		}
	}
	return ""
}

// purposeDiffs summarizes added/removed counts per purpose between two
// versions of a service, for the changelog.
func purposeDiffs(slug string, prev, next map[string]feedschema.Purpose) []feedschema.ServiceChange {
	keys := map[string]bool{}
	for k := range prev {
		keys[k] = true
	}
	for k := range next {
		keys[k] = true
	}
	var out []feedschema.ServiceChange
	for k := range keys {
		prevSet, nextSet := rangeSet(prev[k]), rangeSet(next[k])
		added, removed := 0, 0
		for r := range nextSet {
			if !prevSet[r] {
				added++
			}
		}
		for r := range prevSet {
			if !nextSet[r] {
				removed++
			}
		}
		if added+removed > 0 {
			out = append(out, feedschema.ServiceChange{Slug: slug, Purpose: k, Added: added, Removed: removed})
		}
	}
	return out
}

func rangeSet(p feedschema.Purpose) map[string]bool {
	s := make(map[string]bool, len(p.IPv4)+len(p.IPv6))
	for _, r := range p.IPv4 {
		s[r] = true
	}
	for _, r := range p.IPv6 {
		s[r] = true
	}
	return s
}
