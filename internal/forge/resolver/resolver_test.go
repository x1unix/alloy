package resolver_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/grafana/alloy/internal/forge/manifest"
	"github.com/grafana/alloy/internal/forge/resolver"
)

func TestResolveLocal(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "myreceiver")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write a dummy .go file so it looks like a package.
	if err := os.WriteFile(filepath.Join(srcDir, "receiver.go"), []byte("package myreceiver\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &resolver.Resolver{ModCacheDir: filepath.Join(dir, "cache")}
	src := manifest.SourceConfig{Dir: srcDir}

	result, err := r.Resolve(src, dir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if result.Dir != srcDir {
		t.Errorf("Dir = %q, want %q", result.Dir, srcDir)
	}
	if result.PackageName != "myreceiver" {
		t.Errorf("PackageName = %q, want %q", result.PackageName, "myreceiver")
	}
}

func TestResolveLocal_RelativePath(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "plugins", "myreceiver")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}

	r := &resolver.Resolver{ModCacheDir: filepath.Join(dir, "cache")}
	src := manifest.SourceConfig{Dir: "./myreceiver"}

	result, err := r.Resolve(src, filepath.Join(dir, "plugins"))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if result.Dir != srcDir {
		t.Errorf("Dir = %q, want %q", result.Dir, srcDir)
	}
}

func TestResolveLocal_Missing(t *testing.T) {
	r := &resolver.Resolver{ModCacheDir: t.TempDir()}
	src := manifest.SourceConfig{Dir: "/nonexistent/path"}

	_, err := r.Resolve(src, t.TempDir())
	if err == nil {
		t.Fatal("expected error for missing dir")
	}
}

func TestResolveRemote_CacheHit(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "cache")

	// Pre-populate cache.
	cachedPkg := filepath.Join(cacheDir, "github.com/example/myreceiver@v1.0.0")
	if err := os.MkdirAll(cachedPkg, 0o755); err != nil {
		t.Fatal(err)
	}

	r := &resolver.Resolver{ModCacheDir: cacheDir}
	src := manifest.SourceConfig{
		Import:  "github.com/example/myreceiver",
		Version: "v1.0.0",
	}

	result, err := r.Resolve(src, dir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if result.Dir != cachedPkg {
		t.Errorf("Dir = %q, want %q", result.Dir, cachedPkg)
	}
	if result.Version != "v1.0.0" {
		t.Errorf("Version = %q, want v1.0.0", result.Version)
	}
	if result.PackageName != "myreceiver" {
		t.Errorf("PackageName = %q, want myreceiver", result.PackageName)
	}
}

func TestResolveRemote_LatestResolution(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "cache")

	// Pre-populate cache for the resolved version so download isn't needed.
	cachedPkg := filepath.Join(cacheDir, "github.com/example/myreceiver@v1.2.3")
	if err := os.MkdirAll(cachedPkg, 0o755); err != nil {
		t.Fatal(err)
	}

	// Mock proxy that resolves "latest" to v1.2.3.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/github.com/example/myreceiver/@latest" {
			json.NewEncoder(w).Encode(map[string]string{"Version": "v1.2.3"})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	r := &resolver.Resolver{
		ModCacheDir: cacheDir,
		ProxyURL:    srv.URL,
	}
	src := manifest.SourceConfig{
		Import: "github.com/example/myreceiver",
		// Version empty → resolve latest.
	}

	result, err := r.Resolve(src, dir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if result.Version != "v1.2.3" {
		t.Errorf("Version = %q, want v1.2.3", result.Version)
	}
}
