// Command generator builds the public feed (dist/v1) from feed/sources.yaml.
//
// Usage:
//
//	go run ./generator                                # live fetch
//	go run ./generator -fixtures testdata/fixtures  # offline
//	go run ./generator -services stripe,github        # subset
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

func main() {
	var (
		sourcesPath = flag.String("sources", "sources.yaml", "path to sources.yaml")
		outDir      = flag.String("out", "dist", "output directory (v1/ is created inside)")
		fixturesDir = flag.String("fixtures", "", "offline mode: read bodies from this fixtures directory")
		only        = flag.String("services", "", "comma-separated slugs to build (default: all)")
	)
	flag.Parse()

	reg, err := LoadRegistry(*sourcesPath)
	if err != nil {
		log.Fatalf("load registry: %v", err)
	}

	filter := map[string]bool{}
	for _, s := range strings.Split(*only, ",") {
		if s = strings.TrimSpace(s); s != "" {
			filter[s] = true
		}
	}

	fetcher := NewFetcher(*fixturesDir)
	now := time.Now().UTC()
	generatedAt := now.Format(time.RFC3339)
	syncToken := strconv.FormatInt(now.Unix(), 10)

	v1Dir := filepath.Join(*outDir, "v1")
	svcDir := filepath.Join(v1Dir, "services")
	if err := os.MkdirAll(svcDir, 0o755); err != nil {
		log.Fatal(err)
	}

	var (
		index    feedschema.Index
		failures int
		skipped  int
	)
	index.SchemaVersion = feedschema.SchemaVersion
	index.GeneratedAt = generatedAt
	index.SyncToken = syncToken

	for _, svc := range reg.Services {
		if len(filter) > 0 && !filter[svc.Slug] {
			continue
		}
		out, err := buildService(svc, fetcher, generatedAt, syncToken)
		if err != nil {
			if _, unimpl := err.(errUnimplemented); unimpl {
				log.Printf("SKIP %-16s %v", svc.Slug, err)
				skipped++
			} else {
				log.Printf("FAIL %-16s %v", svc.Slug, err)
				failures++
			}
			continue
		}

		body, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			log.Fatal(err)
		}
		body = append(body, '\n')
		rel := filepath.Join("services", svc.Slug+".json")
		if err := os.WriteFile(filepath.Join(v1Dir, rel), body, 0o644); err != nil {
			log.Fatal(err)
		}

		var purposes []feedschema.PurposeMeta
		for key, p := range out.Purposes {
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
			SHA256:         sha256hex(body),
		})
		total := 0
		for _, p := range out.Purposes {
			total += len(p.IPv4) + len(p.IPv6)
		}
		log.Printf("OK   %-16s %d purposes, %d ranges", svc.Slug, len(out.Purposes), total)
	}

	sort.Slice(index.Services, func(i, j int) bool { return index.Services[i].Slug < index.Services[j].Slug })
	for _, np := range reg.NonPublishers {
		index.NonPublishers = append(index.NonPublishers, feedschema.NonPublisher{
			Slug: np.Slug, Name: np.Name, Evidence: np.Evidence, VendorPosition: np.VendorPosition,
		})
	}

	ib, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	ib = append(ib, '\n')
	if err := os.WriteFile(filepath.Join(v1Dir, "index.json"), ib, 0o644); err != nil {
		log.Fatal(err)
	}

	log.Printf("wrote %s: %d services, %d non-publishers (%d skipped, %d failed)",
		v1Dir, len(index.Services), len(index.NonPublishers), skipped, failures)
	if failures > 0 {
		os.Exit(1)
	}
}

// errUnimplemented marks endpoints whose format has no parser yet (docs-page
// extraction, azure service tags) — skipped, not failed.
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
