package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/slash0-io/feed/feedschema"
)

// previousFeed is the currently published feed, loaded for incremental
// comparison. Nil means "no previous feed exists" and the build publishes
// fresh.
type previousFeed struct {
	index        feedschema.Index
	indexBytes   []byte
	services     map[string]*feedschema.Service
	serviceBytes map[string][]byte
	fingerprints map[string]string
	changelog    []feedschema.ChangelogEntry
}

// previousRetryDelays paces retries of transient previous-feed fetch
// failures (transport errors, 5xx). Tests shorten it.
var previousRetryDelays = []time.Duration{2 * time.Second, 4 * time.Second}

// loadPrevious fetches base (an http(s):// URL or a local directory holding
// index.json). A previous feed that genuinely does not exist (404 / absent
// path) is bootstrap and returns (nil, nil): the build publishes fresh. Any
// other failure fails the build. Publishing fresh against a feed that does
// exist resets every sync token, wipes the changelog, and disables the
// mass-removal guardrail (nothing to compare against), so a transient fetch
// failure must never degrade into it. (2026-07-25: a connection reset on
// this path wiped the published changelog.)
func loadPrevious(base string) (*previousFeed, error) {
	if base == "" {
		return nil, nil
	}
	get := previousGetter(base)

	ib, err := get("index.json")
	if errors.Is(err, fs.ErrNotExist) {
		log.Printf("no previous feed at %s — bootstrap publish", base)
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("previous feed unreachable: %w", err)
	}
	prev := &previousFeed{
		indexBytes:   ib,
		services:     map[string]*feedschema.Service{},
		serviceBytes: map[string][]byte{},
		fingerprints: map[string]string{},
	}
	if err := json.Unmarshal(ib, &prev.index); err != nil {
		return nil, fmt.Errorf("previous index unparseable: %w", err)
	}
	if prev.index.SchemaVersion != feedschema.SchemaVersion {
		return nil, fmt.Errorf("previous index schema %d, want %d", prev.index.SchemaVersion, feedschema.SchemaVersion)
	}
	for _, entry := range prev.index.Services {
		b, err := get(entry.Path)
		if err != nil {
			return nil, fmt.Errorf("previous %s: %w", entry.Slug, err)
		}
		var svc feedschema.Service
		if err := json.Unmarshal(b, &svc); err != nil {
			return nil, fmt.Errorf("previous %s unparseable: %w", entry.Slug, err)
		}
		prev.services[entry.Slug] = &svc
		prev.serviceBytes[entry.Slug] = b
		prev.fingerprints[entry.Slug] = purposesFingerprint(svc.Purposes)
	}
	cb, err := get("changelog.json")
	switch {
	case errors.Is(err, fs.ErrNotExist):
		log.Printf("previous changelog absent — starting empty")
	case err != nil:
		return nil, fmt.Errorf("previous changelog: %w", err)
	default:
		if err := json.Unmarshal(cb, &prev.changelog); err != nil {
			return nil, fmt.Errorf("previous changelog unparseable: %w", err)
		}
	}
	return prev, nil
}

func previousGetter(base string) func(rel string) ([]byte, error) {
	if strings.HasPrefix(base, "http://") || strings.HasPrefix(base, "https://") {
		client := &http.Client{Timeout: 30 * time.Second}
		base = strings.TrimSuffix(base, "/")
		// One cache-busting token per build: a query string gives a distinct
		// CDN cache key (the origin ignores it), so the previous feed is what
		// is published right now rather than a stale cached copy from before
		// the last deploy, and the whole run reads one coherent snapshot.
		bust := "?fresh=" + strconv.FormatInt(time.Now().Unix(), 10)
		// getOnce reports whether a failure is transient (transport error,
		// 5xx, truncated read) and worth retrying. 404 maps to fs.ErrNotExist
		// so callers can tell "does not exist" from "could not fetch".
		getOnce := func(rel string) (body []byte, retryable bool, err error) {
			resp, err := client.Get(base + "/" + rel + bust)
			if err != nil {
				return nil, true, err
			}
			defer resp.Body.Close()
			switch {
			case resp.StatusCode == http.StatusOK:
				b, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
				return b, true, err
			case resp.StatusCode == http.StatusNotFound:
				return nil, false, fmt.Errorf("GET %s/%s: %w", base, rel, fs.ErrNotExist)
			case resp.StatusCode >= 500:
				return nil, true, fmt.Errorf("GET %s/%s: %s", base, rel, resp.Status)
			default:
				return nil, false, fmt.Errorf("GET %s/%s: %s", base, rel, resp.Status)
			}
		}
		return func(rel string) ([]byte, error) {
			for attempt := 0; ; attempt++ {
				b, retryable, err := getOnce(rel)
				if err == nil {
					return b, nil
				}
				if !retryable || attempt >= len(previousRetryDelays) {
					return nil, err
				}
				log.Printf("previous %s: %v — retrying", rel, err)
				time.Sleep(previousRetryDelays[attempt])
			}
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
