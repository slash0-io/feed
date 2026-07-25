package main

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func fastRetries(t *testing.T) {
	t.Helper()
	old := previousRetryDelays
	previousRetryDelays = []time.Duration{0}
	t.Cleanup(func() { previousRetryDelays = old })
}

// A previous feed that exists but cannot be fetched must fail the build:
// degrading to a fresh publish wipes the changelog and disables the
// mass-removal guardrail (this happened in production 2026-07-25).
func TestPreviousServerErrorFailsBuild(t *testing.T) {
	fastRetries(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	if _, err := runBuild(buildOpts(t, fixturesDir, t.TempDir(), srv.URL)); err == nil {
		t.Fatal("build succeeded against an unreachable previous feed; want error")
	}
}

// A previous feed that genuinely does not exist (404) is bootstrap and
// publishes fresh.
func TestPreviousNotFoundBootstraps(t *testing.T) {
	fastRetries(t)
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	s, err := runBuild(buildOpts(t, fixturesDir, t.TempDir(), srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	if s.Fresh == 0 || !s.BuildChanged {
		t.Fatalf("bootstrap build: %+v, want fresh services and BuildChanged", s)
	}
}

// A transient failure recovers within the retry budget and the build
// proceeds incrementally.
func TestPreviousTransientErrorRetries(t *testing.T) {
	fastRetries(t)
	dist1 := t.TempDir()
	if _, err := runBuild(buildOpts(t, fixturesDir, dist1, "")); err != nil {
		t.Fatal(err)
	}

	var calls int32
	files := http.FileServer(http.Dir(dist1))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			http.Error(w, "flake", http.StatusServiceUnavailable)
			return
		}
		files.ServeHTTP(w, r)
	}))
	defer srv.Close()

	s, err := runBuild(buildOpts(t, fixturesDir, t.TempDir(), srv.URL+"/v1"))
	if err != nil {
		t.Fatal(err)
	}
	if s.BuildChanged || s.Unchanged == 0 {
		t.Fatalf("retried build: %+v, want incremental no-op", s)
	}
}
