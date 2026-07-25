package main

import (
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const maxBodyBytes = 96 << 20 // Azure's ServiceTags JSON alone is ~30 MB

// Fetcher retrieves endpoint bodies from the network, or from a fixtures
// directory in offline mode (tests, CI without egress).
type Fetcher struct {
	FixturesDir string // when set, no network access happens
	Client      *http.Client
}

func NewFetcher(fixturesDir string) *Fetcher {
	return &Fetcher{
		FixturesDir: fixturesDir,
		Client:      &http.Client{Timeout: 30 * time.Second},
	}
}

// Get returns the endpoint body and a description of where it came from.
// Fixture lookup tries "<slug>-<endpointID>.data" then "<slug>.data".
func (f *Fetcher) Get(svc SourceService, ep Endpoint) ([]byte, string, error) {
	if f.FixturesDir != "" {
		for _, name := range []string{svc.Slug + "-" + ep.ID, svc.Slug} {
			p := filepath.Join(f.FixturesDir, name+".data")
			if b, err := os.ReadFile(p); err == nil {
				return b, "fixture:" + name, nil
			}
		}
		return nil, "", fmt.Errorf("no fixture for %s/%s in %s", svc.Slug, ep.ID, f.FixturesDir)
	}

	if ep.Format == "azure-service-tags" {
		return f.getAzureServiceTags(ep)
	}

	url := strings.ReplaceAll(ep.URL, "<uuid>", newUUID())
	body, err := f.doGet(url)
	if err != nil {
		return nil, "", err
	}
	return body, ep.URL, nil
}

// fetchRetryDelays paces retries of transient vendor fetch failures. A
// single flaky endpoint otherwise fails its service (kept fail-static) AND
// exits the build nonzero, blocking every other service's updates for the
// whole publish cycle.
var fetchRetryDelays = []time.Duration{2 * time.Second, 4 * time.Second}

// doGet fetches url, retrying transport errors, 5xx, and 429. Other
// statuses (403, 404: the vendor moved or blocked us) fail immediately.
func (f *Fetcher) doGet(url string) ([]byte, error) {
	for attempt := 0; ; attempt++ {
		body, retryable, err := f.getOnce(url)
		if err == nil {
			return body, nil
		}
		if !retryable || attempt >= len(fetchRetryDelays) {
			return nil, err
		}
		log.Printf("fetch %s: %v — retrying", url, err)
		time.Sleep(fetchRetryDelays[attempt])
	}
}

func (f *Fetcher) getOnce(url string) (body []byte, retryable bool, err error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", "slash0-feed-generator/0.1 (+https://github.com/slash0-io/feed)")
	maybeAuthGitHub(req)
	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		retryable := resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests
		return nil, retryable, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	body, err = io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	return body, true, err
}

var reAzureTagsURL = regexp.MustCompile(`https://download\.microsoft\.com/[^"'<> ]*ServiceTags_Public_[0-9]+\.json`)

// getAzureServiceTags resolves Azure's weekly-rotating download URL from the
// Download Center details page, then fetches the ServiceTags JSON itself.
func (f *Fetcher) getAzureServiceTags(ep Endpoint) ([]byte, string, error) {
	page, err := f.doGet(ep.URL)
	if err != nil {
		return nil, "", fmt.Errorf("azure details page: %w", err)
	}
	jsonURL := reAzureTagsURL.FindString(string(page))
	if jsonURL == "" {
		return nil, "", fmt.Errorf("azure details page: no ServiceTags_Public JSON link found")
	}
	body, err := f.doGet(jsonURL)
	if err != nil {
		return nil, "", err
	}
	return body, jsonURL, nil
}

// maybeAuthGitHub authenticates requests to the GitHub API and nothing
// else. Unauthenticated calls share a 60/hour rate limit per IP, and
// Actions runners share IPs, so the github service's meta endpoint 403s
// routinely without this. The token never accompanies other vendors'
// requests.
func maybeAuthGitHub(req *http.Request) {
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" && req.URL.Host == "api.github.com" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
}

func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
