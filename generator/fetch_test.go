package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

func fastFetchRetries(t *testing.T) {
	t.Helper()
	old := fetchRetryDelays
	fetchRetryDelays = []time.Duration{0}
	t.Cleanup(func() { fetchRetryDelays = old })
}

// A transient vendor flake (5xx) retries and recovers instead of failing the
// service and exiting the build nonzero.
func TestFetcherRetriesTransient5xx(t *testing.T) {
	fastFetchRetries(t)
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			http.Error(w, "flake", http.StatusBadGateway)
			return
		}
		w.Write([]byte(`ok`))
	}))
	defer srv.Close()

	f := NewFetcher("")
	body, _, err := f.Get(SourceService{Slug: "x"}, Endpoint{ID: "e", URL: srv.URL})
	if err != nil {
		t.Fatalf("fetch with one 502 flake: %v", err)
	}
	if string(body) != "ok" || calls != 2 {
		t.Fatalf("body=%q calls=%d, want retried success", body, calls)
	}
}

// The CI token goes to the GitHub API and to no other vendor.
func TestGitHubAuthIsHostScoped(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "tok123")
	gh, _ := http.NewRequest(http.MethodGet, "https://api.github.com/meta", nil)
	maybeAuthGitHub(gh)
	if got := gh.Header.Get("Authorization"); got != "Bearer tok123" {
		t.Fatalf("api.github.com auth = %q", got)
	}
	other, _ := http.NewRequest(http.MethodGet, "https://docs.stripe.com/ips", nil)
	maybeAuthGitHub(other)
	if got := other.Header.Get("Authorization"); got != "" {
		t.Fatalf("token leaked to %s: %q", other.URL.Host, got)
	}
}

// 4xx other than 404 means the vendor blocked us or the request is wrong:
// deterministic, so fail immediately rather than hammering them.
func TestFetcherDoesNotRetryOther4xx(t *testing.T) {
	fastFetchRetries(t)
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		http.Error(w, "blocked", http.StatusForbidden)
	}))
	defer srv.Close()

	f := NewFetcher("")
	if _, _, err := f.Get(SourceService{Slug: "x"}, Endpoint{ID: "e", URL: srv.URL}); err == nil {
		t.Fatal("403 must fail")
	}
	if calls != 1 {
		t.Fatalf("403 fetched %d times, want exactly 1", calls)
	}
}

// 404 is the exception among 4xx: a CDN in front of a vendor's docs can serve
// one spuriously. DigitalOcean's geofeed 404ed a publish run on 2026-08-15 and
// served 200 consistently afterwards, so a single 404 should not cost a cycle.
func TestFetcherRetriesSpurious404(t *testing.T) {
	fastFetchRetries(t)
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`ok`))
	}))
	defer srv.Close()

	f := NewFetcher("")
	body, _, err := f.Get(SourceService{Slug: "x"}, Endpoint{ID: "e", URL: srv.URL})
	if err != nil {
		t.Fatalf("fetch with one spurious 404: %v", err)
	}
	if string(body) != "ok" || calls != 2 {
		t.Fatalf("body=%q calls=%d, want retried success", body, calls)
	}
}

// A URL that really is gone still fails, just a few seconds later. Retrying
// must not turn vendor rot into a silent success.
func TestFetcherStillFailsOnPersistent404(t *testing.T) {
	fastFetchRetries(t)
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		http.NotFound(w, r)
	}))
	defer srv.Close()

	f := NewFetcher("")
	if _, _, err := f.Get(SourceService{Slug: "x"}, Endpoint{ID: "e", URL: srv.URL}); err == nil {
		t.Fatal("a persistent 404 must still fail the service")
	}
	// Derived from the retry table rather than hard-coded: the test harness
	// shortens it, so a literal would assert the harness, not the behaviour.
	if want := int32(len(fetchRetryDelays) + 1); calls != want {
		t.Fatalf("persistent 404 fetched %d times, want %d (initial + every retry)", calls, want)
	}
}

// Azure's Download Center answers 200 with the ServiceTags link missing, which
// no status-code retry can catch. It failed publish run #710 on 2026-08-17 and
// served the link correctly minutes later.
func TestAzureRetriesDetailsPageMissingLink(t *testing.T) {
	fastFetchRetries(t)
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/tags.json" {
			w.Write([]byte(`{"values":[]}`))
			return
		}
		// First details-page hit comes back 200 with no link, as Microsoft's
		// does when it flakes.
		if atomic.AddInt32(&calls, 1) == 1 {
			w.Write([]byte(`<html>no link here</html>`))
			return
		}
		w.Write([]byte(`<a href="https://download.microsoft.com/x/ServiceTags_Public_20260810.json">dl</a>`))
	}))
	defer srv.Close()

	f := NewFetcher("")
	// The link the regex accepts must be a real download.microsoft.com URL, so
	// that leg is redirected onto the test server rather than left to escape.
	f.Client.Transport = toTestServer{target: srv.URL + "/tags.json"}

	body, provenance, err := f.getAzureServiceTags(Endpoint{ID: "service-tags", URL: srv.URL})
	if err != nil {
		t.Fatalf("azure fetch with one blank details page: %v", err)
	}
	if calls != 2 {
		t.Fatalf("details page fetched %d times, want 2 (blank then good)", calls)
	}
	if string(body) != `{"values":[]}` {
		t.Fatalf("body = %q, want the ServiceTags JSON", body)
	}
	if provenance == "" {
		t.Fatal("resolved JSON URL must be reported as the provenance URL")
	}
}

// toTestServer redirects the download.microsoft.com leg onto the test server
// and leaves every other request alone, so the details page and the JSON can
// be served by the same handler without any request leaving the process.
type toTestServer struct{ target string }

func (rt toTestServer) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host == "download.microsoft.com" {
		u, err := url.Parse(rt.target)
		if err != nil {
			return nil, err
		}
		req = req.Clone(req.Context())
		req.URL.Scheme, req.URL.Host, req.URL.Path = u.Scheme, u.Host, u.Path
	}
	return http.DefaultTransport.RoundTrip(req)
}

// A details page that has genuinely stopped carrying the link still fails, so
// retrying cannot hide Microsoft restructuring the page.
func TestAzureStillFailsWhenLinkNeverAppears(t *testing.T) {
	fastFetchRetries(t)
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Write([]byte(`<html>restructured, no link</html>`))
	}))
	defer srv.Close()

	f := NewFetcher("")
	if _, _, err := f.getAzureServiceTags(Endpoint{ID: "service-tags", URL: srv.URL}); err == nil {
		t.Fatal("a details page with no link must still fail the service")
	}
	if want := int32(len(fetchRetryDelays) + 1); calls != want {
		t.Fatalf("details page fetched %d times, want %d (initial + every retry)", calls, want)
	}
}
