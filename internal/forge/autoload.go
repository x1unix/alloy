package forge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/grafana/alloy/internal/forge/capsule"
	"github.com/grafana/alloy/internal/forge/manifest"
	"github.com/grafana/alloy/internal/forge/otelreceiver"
	"github.com/grafana/alloy/internal/forge/resolver"
	otelrecv "go.opentelemetry.io/collector/receiver"
)

// LoadConfig holds configuration for the Forge loader.
type LoadConfig struct {
	// PluginsDir is a path to load plugins from.
	PluginsDir string

	// ModCacheDir is a path to cache Go module dependencies.
	ModCacheDir string
}

// Load scans PluginsDir for manifest files, resolves their sources, evaluates
// factory snippets via Yaegi, and registers the resulting components.
func Load(cfg LoadConfig) error {
	entries, err := os.ReadDir(cfg.PluginsDir)
	if err != nil {
		return fmt.Errorf("read plugins dir %s: %w", cfg.PluginsDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml") {
			continue
		}

		path := filepath.Join(cfg.PluginsDir, name)
		if err := loadPlugin(path, cfg); err != nil {
			return fmt.Errorf("load plugin %s: %w", name, err)
		}
	}
	return nil
}

func loadPlugin(path string, cfg LoadConfig) error {
	// Step 1: Parse and validate the manifest.
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open manifest: %w", err)
	}
	defer f.Close()

	m, err := manifest.Parse(f)
	if err != nil {
		return fmt.Errorf("parse manifest: %w", err)
	}
	if err := m.Validate(); err != nil {
		return fmt.Errorf("validate manifest: %w", err)
	}

	// Step 3: Resolve the package source.
	res := &resolver.Resolver{
		ModCacheDir: cfg.ModCacheDir,
	}
	result, err := res.Resolve(m.Source, filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("resolve source: %w", err)
	}

	// Step 2: Set up the Yaegi interpreter.
	// Use the module cache as GOPATH so Yaegi can find downloaded packages.
	goPath := cfg.ModCacheDir
	if m.Source.Dir != "" {
		// For local sources, use the parent of the source dir.
		goPath = filepath.Dir(result.Dir)
	}

	cap, err := capsule.New(goPath)
	if err != nil {
		return fmt.Errorf("create interpreter: %w", err)
	}

	// Determine the import path and package name.
	importPath := result.ImportPath
	if importPath == "" {
		importPath = m.Source.Import
	}
	pkgName := result.PackageName

	// Evaluate the factory snippet.
	switch m.Type {
	case manifest.ComponentTypeOtelReceiver:
		return loadOtelReceiver(m, cap, importPath, pkgName)
	default:
		return fmt.Errorf("unsupported component type %q", m.Type)
	}
}

func loadOtelReceiver(m *manifest.Manifest, cap *capsule.Capsule, importPath, pkgName string) error {
	factoryAny, err := cap.EvalFactory(importPath, pkgName, m.OtelReceiver.Factory)
	if err != nil {
		return fmt.Errorf("eval factory: %w", err)
	}

	factory, ok := factoryAny.(otelrecv.Factory)
	if !ok {
		return fmt.Errorf("factory returned %T, want receiver.Factory", factoryAny)
	}

	if err := otelreceiver.Register(m, factory); err != nil {
		return fmt.Errorf("register component: %w", err)
	}

	return nil
}
