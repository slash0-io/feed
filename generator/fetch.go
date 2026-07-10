package main

import (
	"crypto/rand"
	"fmt"
	"io"
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
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "egress-feed-generator/0.1 (+https://github.com/egresshq/feed)")
	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, "", err
	}
	return body, ep.URL, nil
}

var reAzureTagsURL = regexp.MustCompile(`https://download\.microsoft\.com/[^"'<> ]*ServiceTags_Public_[0-9]+\.json`)

// getAzureServiceTags resolves Azure's weekly-rotating download URL from the
// Download Center details page, then fetches the ServiceTags JSON itself.
func (f *Fetcher) getAzureServiceTags(ep Endpoint) ([]byte, string, error) {
	page, _, err := f.rawGet(ep.URL)
	if err != nil {
		return nil, "", fmt.Errorf("azure details page: %w", err)
	}
	jsonURL := reAzureTagsURL.FindString(string(page))
	if jsonURL == "" {
		return nil, "", fmt.Errorf("azure details page: no ServiceTags_Public JSON link found")
	}
	body, _, err := f.rawGet(jsonURL)
	if err != nil {
		return nil, "", err
	}
	return body, jsonURL, nil
}

func (f *Fetcher) rawGet(url string) ([]byte, string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "egress-feed-generator/0.1 (+https://github.com/egresshq/feed)")
	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	return body, url, err
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
