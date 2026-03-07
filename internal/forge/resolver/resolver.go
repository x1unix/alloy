// Package resolver locates or downloads Go packages referenced by forge manifests.
// It uses the Go toolchain to resolve transitive dependencies and builds
// a GOPATH-compatible source tree that Yaegi can interpret.
package resolver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/grafana/alloy/internal/forge/manifest"
)

// Result holds the resolved package location.
type Result struct {
	// Dir is the absolute filesystem path to the package source.
	Dir string

	// PackageName is the short Go package name for imports.
	PackageName string

	// ImportPath is the full Go module import path.
	ImportPath string

	// Version is the resolved module version (empty for local dirs).
	Version string

	// GoPath is the GOPATH root containing the resolved source tree.
	// Yaegi should be configured with this path.
	GoPath string
}

// Resolver fetches or locates Go packages for forge plugins.
type Resolver struct {
	// ModCacheDir is the root directory for cached module downloads
	// and the GOPATH source tree.
	ModCacheDir string
}

// Resolve locates the package described by src and downloads all transitive
// dependencies. baseDir is used to resolve relative source.dir paths.
// The returned Result includes a GoPath suitable for Yaegi interpretation.
func (r *Resolver) Resolve(src manifest.SourceConfig, baseDir string) (*Result, error) {
	if src.Dir != "" {
		return r.resolveLocal(src, baseDir)
	}
	return r.resolveRemote(src)
}

func (r *Resolver) resolveLocal(src manifest.SourceConfig, baseDir string) (*Result, error) {
	dir := src.Dir
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(baseDir, dir)
	}

	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve local source: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("source.dir %q is not a directory", dir)
	}

	pkgName := src.PackageName
	if pkgName == "" {
		pkgName = filepath.Base(dir)
	}

	// For local sources with an import path, build a GOPATH tree that
	// includes the local dir and all of its transitive dependencies.
	var goPath string
	if src.Import != "" {
		var err error
		goPath, err = r.buildGoPath(src, dir)
		if err != nil {
			return nil, fmt.Errorf("build gopath: %w", err)
		}
	}

	return &Result{
		Dir:         dir,
		PackageName: pkgName,
		ImportPath:  src.Import,
		GoPath:      goPath,
	}, nil
}

func (r *Resolver) resolveRemote(src manifest.SourceConfig) (*Result, error) {
	pkgName := src.PackageName
	if pkgName == "" {
		pkgName = path.Base(src.Import)
	}

	goPath, err := r.buildGoPath(src, "")
	if err != nil {
		return nil, fmt.Errorf("build gopath: %w", err)
	}

	return &Result{
		PackageName: pkgName,
		ImportPath:  src.Import,
		GoPath:      goPath,
	}, nil
}

// buildGoPath creates a GOPATH-compatible source tree for the given package
// and all its transitive dependencies. It creates a temporary Go module,
// runs `go mod download` to fetch everything, then copies source files
// into a gopath/src/ layout that Yaegi can interpret.
//
// Go modules often have overlapping path prefixes (e.g.,
// `collector/component` and `collector/component/componenttest` are
// separate modules). Simple symlinking fails because a symlinked parent
// directory points to a read-only module cache. To handle this robustly,
// we copy source files from each module into the GOPATH tree.
func (r *Resolver) buildGoPath(src manifest.SourceConfig, localDir string) (string, error) {
	goPathRoot := filepath.Join(r.ModCacheDir, "gopath")
	srcRoot := filepath.Join(goPathRoot, "src")

	// Create a temporary module to resolve dependencies.
	tmpDir, err := os.MkdirTemp("", "forge-resolve-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := writeResolverModule(tmpDir, src, localDir); err != nil {
		return "", err
	}

	// Download all dependencies.
	if err := runGo(tmpDir, "mod", "download", "-x"); err != nil {
		return "", fmt.Errorf("go mod download: %w", err)
	}

	// Get the full resolved module list.
	modules, err := listModules(tmpDir)
	if err != nil {
		return "", err
	}

	// Copy each module's source into the GOPATH tree.
	for _, m := range modules {
		if m.Main || m.Dir == "" {
			continue
		}

		destDir := filepath.Join(srcRoot, m.Path)
		if err := copyModuleSource(m.Dir, destDir); err != nil {
			return "", fmt.Errorf("copy module %s: %w", m.Path, err)
		}
	}

	// For local sources, also copy the package itself.
	if localDir != "" && src.Import != "" {
		destDir := filepath.Join(srcRoot, src.Import)
		if err := copyModuleSource(localDir, destDir); err != nil {
			return "", fmt.Errorf("copy local source: %w", err)
		}
	}

	return goPathRoot, nil
}

// copyModuleSource recursively copies source files from srcDir into destDir.
// Only copies .go files and preserves directory structure. Existing files
// are overwritten but existing directories from other modules are preserved.
func copyModuleSource(srcDir, destDir string) error {
	return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(destDir, rel)

		if d.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}

		// Only copy .go source files — Yaegi interprets from source.
		if !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dest, data, 0o644)
	})
}

type modEntry struct {
	Path    string `json:"Path"`
	Version string `json:"Version"`
	Dir     string `json:"Dir"`
	Main    bool   `json:"Main"`
}

func listModules(dir string) ([]modEntry, error) {
	cmd := exec.Command("go", "list", "-m", "-json", "all")
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list -m -json all: %s: %w", stderr.String(), err)
	}

	var modules []modEntry
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var m modEntry
		if err := dec.Decode(&m); err != nil {
			break
		}
		modules = append(modules, m)
	}
	return modules, nil
}

func writeResolverModule(dir string, src manifest.SourceConfig, localDir string) error {
	var goMod strings.Builder
	goMod.WriteString("module forge-resolve\n\ngo 1.24\n\n")

	if localDir != "" && src.Import != "" {
		fmt.Fprintf(&goMod, "require %s v0.0.0\n", src.Import)
		fmt.Fprintf(&goMod, "replace %s => %s\n", src.Import, localDir)
	} else if src.Version != "" && src.Version != "latest" {
		fmt.Fprintf(&goMod, "require %s %s\n", src.Import, src.Version)
	} else {
		fmt.Fprintf(&goMod, "require %s latest\n", src.Import)
	}

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod.String()), 0o644); err != nil {
		return fmt.Errorf("write go.mod: %w", err)
	}

	// Write a minimal .go file that imports the target package
	// so `go mod tidy` resolves it.
	mainGo := fmt.Sprintf("package main\n\nimport _ \"%s\"\n", src.Import)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(mainGo), 0o644); err != nil {
		return fmt.Errorf("write main.go: %w", err)
	}

	// Run go mod tidy to resolve the actual version and transitive deps.
	if err := runGo(dir, "mod", "tidy"); err != nil {
		return fmt.Errorf("go mod tidy: %w", err)
	}

	return nil
}

// runGo executes a go command in the given directory.
func runGo(dir string, args ...string) error {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go %s: %s: %w", strings.Join(args, " "), stderr.String(), err)
	}
	return nil
}
