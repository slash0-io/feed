package main

import (
	"net/http"
	"net/http/httptest"
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

// 4xx means the vendor moved or blocked us: fail immediately, no retries.
func TestFetcherDoesNotRetry4xx(t *testing.T) {
	fastFetchRetries(t)
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		http.NotFound(w, r)
	}))
	defer srv.Close()

	f := NewFetcher("")
	if _, _, err := f.Get(SourceService{Slug: "x"}, Endpoint{ID: "e", URL: srv.URL}); err == nil {
		t.Fatal("404 must fail")
	}
	if calls != 1 {
		t.Fatalf("404 fetched %d times, want exactly 1", calls)
	}
}
