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

// Load scans PluginsDir for manifest files, resolves their sources,
// builds factory plugins, and registers the resulting components.
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

	res := &resolver.Resolver{
		ModCacheDir: cfg.ModCacheDir,
	}
	result, err := res.Resolve(m.Source, filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("resolve source: %w", err)
	}

	switch m.Type {
	case manifest.ComponentTypeOtelReceiver:
		return loadOtelReceiver(m, result)
	default:
		return fmt.Errorf("unsupported component type %q", m.Type)
	}
}

func loadOtelReceiver(m *manifest.Manifest, result *resolver.Result) error {
	// Create a Yaegi interpreter with the resolved GOPATH.
	cap, err := capsule.New(result.GoPath)
	if err != nil {
		return fmt.Errorf("create capsule: %w", err)
	}

	// Evaluate the factory snippet using Yaegi.
	factoryAny, err := cap.EvalFactory(result.ImportPath, result.PackageName, m.OtelReceiver.Factory)
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
