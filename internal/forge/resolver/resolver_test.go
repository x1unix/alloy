package resolver_test

import (
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
