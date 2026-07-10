// Command generator builds the public feed (dist/v1) from sources.yaml.
//
// Usage:
//
//	go run ./generator                                 # live fetch, fresh publish
//	go run ./generator -fixtures testdata/fixtures     # offline
//	go run ./generator -previous https://egresshq.github.io/feed/v1
//	go run ./generator -services stripe,github         # dev subset (don't combine with -previous)
//
// With -previous, publishing is incremental: services whose normalized range
// set is unchanged are republished byte-for-byte (sync tokens and timestamps
// preserved), real changes are recorded in dist/v1/changelog.json, and a
// change that removes most of a service's previously published ranges is
// quarantined — the previous version keeps serving and the build exits
// nonzero for humans to review. <out>/BUILD_CHANGED (true|false) tells CI
// whether deploying is worthwhile.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/egresshq/feed/feedschema"
)

type buildOptions struct {
	SourcesPath    string
	OutDir         string
	FixturesDir    string
	Only           string
	Previous       string
	MaxRemovedFrac float64
	MinGuardCount  int
}

type buildStats struct {
	Fresh, Unchanged, Changed, Skipped, Failed, Quarantined int
	BuildChanged                                            bool
}

func main() {
	var opts buildOptions
	flag.StringVar(&opts.SourcesPath, "sources", "sources.yaml", "path to sources.yaml")
	flag.StringVar(&opts.OutDir, "out", "dist", "output directory (v1/ is created inside)")
	flag.StringVar(&opts.FixturesDir, "fixtures", "", "offline mode: read bodies from this fixtures directory")
	flag.StringVar(&opts.Only, "services", "", "comma-separated slugs to build (default: all)")
	flag.StringVar(&opts.Previous, "previous", "", "published feed (http(s):// URL or directory) to publish incrementally against")
	flag.Float64Var(&opts.MaxRemovedFrac, "max-removed-frac", 0.5, "quarantine a change removing more than this fraction of a purpose's ranges")
	flag.IntVar(&opts.MinGuardCount, "min-guard-count", 8, "apply the removal guardrail only when the purpose previously had at least this many ranges")
	flag.Parse()

	stats, err := runBuild(opts)
	if err != nil {
		log.Fatal(err)
	}
	if stats.Failed > 0 || stats.Quarantined > 0 {
		os.Exit(1)
	}
}

func runBuild(opts buildOptions) (buildStats, error) {
	var stats buildStats
	reg, err := LoadRegistry(opts.SourcesPath)
	if err != nil {
		return stats, fmt.Errorf("load registry: %w", err)
	}

	filter := map[string]bool{}
	for _, s := range strings.Split(opts.Only, ",") {
		if s = strings.TrimSpace(s); s != "" {
			filter[s] = true
		}
	}

	prev := loadPrevious(opts.Previous)
	fetcher := NewFetcher(opts.FixturesDir)
	now := time.Now().UTC()
	generatedAt := now.Format(time.RFC3339)
	syncToken := strconv.FormatInt(now.Unix(), 10)

	v1Dir := filepath.Join(opts.OutDir, "v1")
	if err := os.MkdirAll(filepath.Join(v1Dir, "services"), 0o755); err != nil {
		return stats, err
	}

	var (
		index   feedschema.Index
		changes []feedschema.ServiceChange
	)
	index.SchemaVersion = feedschema.SchemaVersion

	for _, svc := range reg.Services {
		if len(filter) > 0 && !filter[svc.Slug] {
			continue
		}
		built, buildErr := buildService(svc, fetcher, generatedAt, syncToken)

		// Decide what actually gets published for this service.
		var doc *feedschema.Service
		var docBytes []byte
		switch {
		case buildErr != nil:
			if _, unimpl := buildErr.(errUnimplemented); unimpl {
				log.Printf("SKIP %-16s %v", svc.Slug, buildErr)
				stats.Skipped++
			} else {
				log.Printf("FAIL %-16s %v", svc.Slug, buildErr)
				stats.Failed++
			}
			// Fail static: a broken or unreachable upstream must not shrink
			// the published catalog — keep serving the last-good version.
			if prev == nil || prev.services[svc.Slug] == nil {
				continue
			}
			doc, docBytes = prev.services[svc.Slug], prev.serviceBytes[svc.Slug]
			log.Printf("KEEP %-16s serving previously published version", svc.Slug)
		case prev != nil && prev.fingerprints[svc.Slug] == purposesFingerprint(built.Purposes):
			doc, docBytes = prev.services[svc.Slug], prev.serviceBytes[svc.Slug]
			stats.Unchanged++
			log.Printf("OK   %-16s unchanged", svc.Slug)
		case prev != nil && prev.services[svc.Slug] != nil:
			if reason := quarantineReason(prev.services[svc.Slug].Purposes, built.Purposes, opts.MaxRemovedFrac, opts.MinGuardCount); reason != "" {
				doc, docBytes = prev.services[svc.Slug], prev.serviceBytes[svc.Slug]
				stats.Quarantined++
				log.Printf("QUAR %-16s %s — keeping previously published version", svc.Slug, reason)
				break
			}
			diffs := purposeDiffs(svc.Slug, prev.services[svc.Slug].Purposes, built.Purposes)
			changes = append(changes, diffs...)
			doc = built
			if docBytes, err = marshalDoc(built); err != nil {
				return stats, err
			}
			stats.Changed++
			log.Printf("OK   %-16s changed: %s", svc.Slug, summarizeDiffs(diffs))
		default:
			if prev != nil {
				changes = append(changes, purposeDiffs(svc.Slug, nil, built.Purposes)...)
			}
			doc = built
			if docBytes, err = marshalDoc(built); err != nil {
				return stats, err
			}
			stats.Fresh++
			total := 0
			for _, p := range built.Purposes {
				total += len(p.IPv4) + len(p.IPv6)
			}
			log.Printf("OK   %-16s %d purposes, %d ranges", svc.Slug, len(built.Purposes), total)
		}

		rel := filepath.Join("services", svc.Slug+".json")
		if err := os.WriteFile(filepath.Join(v1Dir, rel), docBytes, 0o644); err != nil {
			return stats, err
		}
		var purposes []feedschema.PurposeMeta
		for key, p := range doc.Purposes {
			purposes = append(purposes, feedschema.PurposeMeta{Key: key, Direction: p.Direction})
		}
		sort.Slice(purposes, func(i, j int) bool { return purposes[i].Key < purposes[j].Key })
		index.Services = append(index.Services, feedschema.IndexService{
			Slug:           svc.Slug,
			Name:           svc.Name,
			Category:       svc.Category,
			Classification: svc.Classification,
			Purposes:       purposes,
			Path:           filepath.ToSlash(rel),
			SHA256:         sha256hex(docBytes),
		})
	}

	sort.Slice(index.Services, func(i, j int) bool { return index.Services[i].Slug < index.Services[j].Slug })
	for _, np := range reg.NonPublishers {
		index.NonPublishers = append(index.NonPublishers, feedschema.NonPublisher{
			Slug: np.Slug, Name: np.Name, Evidence: np.Evidence, VendorPosition: np.VendorPosition,
		})
	}

	// Publish the index and changelog. If the catalog is identical to the
	// previous publish, reuse its bytes verbatim so the whole dist tree is
	// byte-stable and CI can skip deployment.
	stats.BuildChanged = prev == nil || len(changes) > 0 || !indexEquivalent(&index, &prev.index)
	var indexBytes, changelogBytes []byte
	if !stats.BuildChanged {
		indexBytes = prev.indexBytes
		if changelogBytes, err = marshalPretty(orEmpty(prev.changelog)); err != nil {
			return stats, err
		}
	} else {
		index.GeneratedAt = generatedAt
		index.SyncToken = syncToken
		if indexBytes, err = marshalPretty(index); err != nil {
			return stats, err
		}
		changelog := orEmpty(nil)
		if prev != nil {
			changelog = orEmpty(prev.changelog)
		}
		if len(changes) > 0 {
			sort.Slice(changes, func(i, j int) bool {
				if changes[i].Slug != changes[j].Slug {
					return changes[i].Slug < changes[j].Slug
				}
				return changes[i].Purpose < changes[j].Purpose
			})
			changelog = append([]feedschema.ChangelogEntry{{
				PublishedAt: generatedAt,
				SyncToken:   syncToken,
				Changes:     changes,
			}}, changelog...)
			if len(changelog) > 500 {
				changelog = changelog[:500]
			}
		}
		if changelogBytes, err = marshalPretty(changelog); err != nil {
			return stats, err
		}
	}
	if err := os.WriteFile(filepath.Join(v1Dir, "index.json"), indexBytes, 0o644); err != nil {
		return stats, err
	}
	if err := os.WriteFile(filepath.Join(v1Dir, "changelog.json"), changelogBytes, 0o644); err != nil {
		return stats, err
	}
	if err := os.WriteFile(filepath.Join(opts.OutDir, "BUILD_CHANGED"), []byte(fmt.Sprintf("%v\n", stats.BuildChanged)), 0o644); err != nil {
		return stats, err
	}

	log.Printf("wrote %s: %d services (%d fresh, %d unchanged, %d changed, %d quarantined), %d non-publishers, %d skipped, %d failed — changed=%v",
		v1Dir, len(index.Services), stats.Fresh, stats.Unchanged, stats.Changed, stats.Quarantined,
		len(index.NonPublishers), stats.Skipped, stats.Failed, stats.BuildChanged)
	return stats, nil
}

// indexEquivalent compares everything except the publish tokens.
func indexEquivalent(a, b *feedschema.Index) bool {
	ac, bc := *a, *b
	ac.GeneratedAt, ac.SyncToken = "", ""
	bc.GeneratedAt, bc.SyncToken = "", ""
	ab, _ := json.Marshal(ac)
	bb, _ := json.Marshal(bc)
	return string(ab) == string(bb)
}

func summarizeDiffs(diffs []feedschema.ServiceChange) string {
	parts := make([]string, 0, len(diffs))
	for _, d := range diffs {
		parts = append(parts, fmt.Sprintf("%s +%d/-%d", d.Purpose, d.Added, d.Removed))
	}
	return strings.Join(parts, ", ")
}

func orEmpty(c []feedschema.ChangelogEntry) []feedschema.ChangelogEntry {
	if c == nil {
		return []feedschema.ChangelogEntry{}
	}
	return c
}

func marshalDoc(v any) ([]byte, error) { return marshalPretty(v) }

func marshalPretty(v any) ([]byte, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// errUnimplemented marks endpoints whose format has no parser yet — skipped,
// not failed.
type errUnimplemented struct{ format string }

func (e errUnimplemented) Error() string { return "format not yet implemented: " + e.format }

// buildService fetches all endpoints of a service and assembles its feed
// document. A service with zero parseable endpoints returns an error.
func buildService(svc SourceService, fetcher *Fetcher, generatedAt, syncToken string) (*feedschema.Service, error) {
	out := &feedschema.Service{
		SchemaVersion:  feedschema.SchemaVersion,
		Slug:           svc.Slug,
		Name:           svc.Name,
		Category:       svc.Category,
		Classification: svc.Classification,
		GeneratedAt:    generatedAt,
		SyncToken:      syncToken,
		Provenance:     svc.Provenance,
		Purposes:       map[string]feedschema.Purpose{},
	}

	var lastUnimplemented *errUnimplemented
	for _, ep := range svc.Endpoints {
		parse, ok := parsers[ep.Format]
		if !ok {
			lastUnimplemented = &errUnimplemented{ep.Format}
			continue
		}
		body, from, err := fetcher.Get(svc, ep)
		if err != nil {
			return nil, fmt.Errorf("endpoint %s: %w", ep.ID, err)
		}
		out.Sources = append(out.Sources, feedschema.SourceRecord{
			URL:         from,
			RetrievedAt: generatedAt,
			SHA256:      sha256hex(body),
		})
		for _, decl := range ep.Purposes {
			raw, err := parse(body, decl.Select)
			if err != nil {
				return nil, fmt.Errorf("endpoint %s purpose %s: %w", ep.ID, decl.Key, err)
			}
			warn := func(format string, args ...any) {
				log.Printf("warn %s/%s: %s", svc.Slug, decl.Key, fmt.Sprintf(format, args...))
			}
			v4, v6 := normalize(raw, warn)
			if len(v4)+len(v6) == 0 {
				return nil, fmt.Errorf("endpoint %s purpose %s: produced zero ranges (guardrail)", ep.ID, decl.Key)
			}
			out.Purposes[decl.Key] = feedschema.Purpose{Direction: decl.Direction, IPv4: v4, IPv6: v6}
		}
	}

	if len(out.Purposes) == 0 {
		if lastUnimplemented != nil {
			return nil, *lastUnimplemented
		}
		return nil, fmt.Errorf("no purposes produced")
	}
	return out, nil
}

func sha256hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
