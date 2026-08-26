package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/slash0-io/feed/feedschema"
	"io"
	"io/fs"
	"log"
	"math/big"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
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
		// This token does NOT bust the CDN, despite reading like it should.
		// Measured 2026-08-26 against feed.slash0.io: a random ?fresh= value
		// returns "x-cache: HIT" with the same age as the plain URL, and
		// Cache-Control: no-cache, max-age=0, no-store and Pragma: no-cache
		// are all ignored the same way. What actually makes the previous feed
		// current is the CDN purge Pages performs on deploy. Kept only so the
		// request shape does not change; do not rely on it for freshness.
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

// quarantineReason applies the mass-removal guardrail: a change that stops
// publishing more than maxFrac of a purpose's previously published ADDRESS
// SPACE (when the purpose had at least minCount ranges) must not auto-publish,
// because a truncated or erroneous upstream body would otherwise propagate to
// every consumer.
//
// Coverage is compared as address sets, never as CIDR strings. A vendor is
// free to re-express the same space with different prefixes, and doing so
// removes almost every old string while removing no addresses at all. Azure
// did exactly that on 2026-08-24, summarising the AzureCloud service tag from
// 15385 prefixes into 2016 broader supernets: a string comparison called it
// "2765 of 4540 ranges removed (61%)" and quarantined the service for two
// days, while the new set was in fact a strict superset of the old one.
func quarantineReason(prev, next map[string]feedschema.Purpose, maxFrac float64, minCount int) string {
	for key, pp := range prev {
		if len(pp.IPv4)+len(pp.IPv6) < minCount {
			continue
		}
		np, ok := next[key]
		if !ok {
			return fmt.Sprintf("purpose %q disappeared (%d ranges)", key, len(pp.IPv4)+len(pp.IPv6))
		}
		// Families are judged separately: one IPv6 range outweighs the whole
		// IPv4 internet by address count, so a combined fraction would hide
		// the total loss of a purpose's IPv4 coverage.
		for _, fam := range []struct {
			name       string
			prev, next []string
		}{
			{"IPv4", pp.IPv4, np.IPv4},
			{"IPv6", pp.IPv6, np.IPv6},
		} {
			had, lost := lostCoverage(fam.prev, fam.next)
			if had.Sign() == 0 {
				continue
			}
			frac := new(big.Rat).SetFrac(lost, had)
			f, _ := frac.Float64()
			if f > maxFrac {
				return fmt.Sprintf("purpose %q: %.0f%% of previously published %s addresses no longer covered (%s of %s)",
					key, f*100, fam.name, lost, had)
			}
		}
	}
	return ""
}

// lostCoverage reports how much of prev's address space next does not cover,
// alongside the size of prev. Both are address counts, so an IPv4 and an IPv6
// list must not be mixed in one call.
func lostCoverage(prev, next []string) (had, lost *big.Int) {
	pv := coalesceIntervals(intervals(prev))
	nx := coalesceIntervals(intervals(next))
	had, lost = new(big.Int), new(big.Int)
	one := big.NewInt(1)
	for _, iv := range pv {
		size := new(big.Int).Sub(iv[1], iv[0])
		size.Add(size, one)
		had.Add(had, size)
		// nx is sorted and disjoint, so walk it once per prev interval and
		// subtract each overlap.
		uncovered := new(big.Int).Set(size)
		for _, n := range nx {
			if n[1].Cmp(iv[0]) < 0 {
				continue
			}
			if n[0].Cmp(iv[1]) > 0 {
				break
			}
			lo, hi := iv[0], iv[1]
			if n[0].Cmp(lo) > 0 {
				lo = n[0]
			}
			if n[1].Cmp(hi) < 0 {
				hi = n[1]
			}
			overlap := new(big.Int).Sub(hi, lo)
			overlap.Add(overlap, one)
			uncovered.Sub(uncovered, overlap)
		}
		if uncovered.Sign() > 0 {
			lost.Add(lost, uncovered)
		}
	}
	return had, lost
}

// intervals converts CIDR strings to inclusive [lo, hi] address ranges.
// Anything unparseable is skipped: normalize has already rejected it upstream,
// and a parse failure here must not be read as lost coverage.
func intervals(cidrs []string) [][2]*big.Int {
	out := make([][2]*big.Int, 0, len(cidrs))
	for _, c := range cidrs {
		p, err := netip.ParsePrefix(c)
		if err != nil {
			continue
		}
		p = p.Masked()
		var b []byte
		if p.Addr().Is4() {
			a := p.Addr().As4()
			b = a[:]
		} else {
			a := p.Addr().As16()
			b = a[:]
		}
		lo := new(big.Int).SetBytes(b)
		size := new(big.Int).Lsh(big.NewInt(1), uint(len(b)*8-p.Bits()))
		hi := new(big.Int).Add(lo, size)
		hi.Sub(hi, big.NewInt(1))
		out = append(out, [2]*big.Int{lo, hi})
	}
	return out
}

// coalesceIntervals sorts and coalesces overlapping or adjacent ranges so the
// sweep in lostCoverage can never count the same address twice.
func coalesceIntervals(in [][2]*big.Int) [][2]*big.Int {
	if len(in) == 0 {
		return in
	}
	sort.Slice(in, func(i, j int) bool { return in[i][0].Cmp(in[j][0]) < 0 })
	out := [][2]*big.Int{in[0]}
	one := big.NewInt(1)
	for _, iv := range in[1:] {
		last := out[len(out)-1]
		next := new(big.Int).Add(last[1], one)
		if iv[0].Cmp(next) <= 0 {
			if iv[1].Cmp(last[1]) > 0 {
				last[1] = iv[1]
			}
			continue
		}
		out = append(out, iv)
	}
	return out
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
