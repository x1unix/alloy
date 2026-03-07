// Package resolver locates or downloads Go packages referenced by forge manifests.
package resolver

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/grafana/alloy/internal/forge/manifest"
)

const defaultProxy = "https://proxy.golang.org"

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
}

// Resolver fetches or locates Go packages for forge plugins.
type Resolver struct {
	// ModCacheDir is the root directory for cached module downloads.
	ModCacheDir string

	// ProxyURL is the Go module proxy base URL.
	// Defaults to https://proxy.golang.org.
	ProxyURL string

	// Client is the HTTP client used for proxy requests.
	// Defaults to http.DefaultClient.
	Client *http.Client
}

func (r *Resolver) proxyURL() string {
	if r.ProxyURL != "" {
		return r.ProxyURL
	}
	if env := os.Getenv("GOPROXY"); env != "" {
		// Take the first proxy from a comma-separated list.
		if i := strings.IndexByte(env, ','); i > 0 {
			return env[:i]
		}
		return env
	}
	return defaultProxy
}

func (r *Resolver) client() *http.Client {
	if r.Client != nil {
		return r.Client
	}
	return http.DefaultClient
}

// Resolve locates the package described by src.
// baseDir is used to resolve relative source.dir paths.
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

	return &Result{
		Dir:         dir,
		PackageName: pkgName,
	}, nil
}

func (r *Resolver) resolveRemote(src manifest.SourceConfig) (*Result, error) {
	version := src.Version
	if version == "" || version == "latest" {
		v, err := r.resolveLatest(src.Import)
		if err != nil {
			return nil, err
		}
		version = v
	}

	pkgName := src.PackageName
	if pkgName == "" {
		pkgName = path.Base(src.Import)
	}

	// Check cache.
	cacheDir := filepath.Join(r.ModCacheDir, encodePath(src.Import)+"@"+version)
	if info, err := os.Stat(cacheDir); err == nil && info.IsDir() {
		return &Result{
			Dir:         cacheDir,
			PackageName: pkgName,
			ImportPath:  src.Import,
			Version:     version,
		}, nil
	}

	// Download from proxy.
	if err := r.download(src.Import, version, cacheDir); err != nil {
		return nil, fmt.Errorf("download %s@%s: %w", src.Import, version, err)
	}

	return &Result{
		Dir:         cacheDir,
		PackageName: pkgName,
		ImportPath:  src.Import,
		Version:     version,
	}, nil
}

type versionInfo struct {
	Version string `json:"Version"`
}

func (r *Resolver) resolveLatest(modulePath string) (string, error) {
	url := r.proxyURL() + "/" + encodePath(modulePath) + "/@latest"
	resp, err := r.client().Get(url)
	if err != nil {
		return "", fmt.Errorf("resolve latest version for %s: %w", modulePath, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("resolve latest version for %s: HTTP %d", modulePath, resp.StatusCode)
	}

	var info versionInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", fmt.Errorf("decode version info for %s: %w", modulePath, err)
	}
	if info.Version == "" {
		return "", fmt.Errorf("empty version in response for %s", modulePath)
	}
	return info.Version, nil
}

func (r *Resolver) download(modulePath, version, destDir string) error {
	url := r.proxyURL() + "/" + encodePath(modulePath) + "/@v/" + version + ".zip"
	resp, err := r.client().Get(url)
	if err != nil {
		return fmt.Errorf("fetch zip: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch zip: HTTP %d", resp.StatusCode)
	}

	// Write zip to a temp file so we can open it with zip.OpenReader.
	tmpFile, err := os.CreateTemp("", "forge-module-*.zip")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		return fmt.Errorf("download zip: %w", err)
	}
	tmpFile.Close()

	return extractZip(tmpFile.Name(), modulePath, version, destDir)
}

// extractZip extracts a Go module zip archive into destDir.
// Go module zips have a top-level directory of <module>@<version>/ which we strip.
func extractZip(zipPath, modulePath, version, destDir string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer zr.Close()

	prefix := modulePath + "@" + version + "/"

	for _, f := range zr.File {
		name := f.Name
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		relPath := strings.TrimPrefix(name, prefix)
		if relPath == "" {
			continue
		}

		target := filepath.Join(destDir, filepath.FromSlash(relPath))

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}

		if err := extractFile(f, target); err != nil {
			return err
		}
	}

	return nil
}

func extractFile(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.Create(target)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return errors.Join(err, out.Close())
}

// encodePath encodes a module path for use in proxy URLs.
// Upper case letters are replaced with !<lower>.
func encodePath(modPath string) string {
	var b strings.Builder
	for _, r := range modPath {
		if 'A' <= r && r <= 'Z' {
			b.WriteByte('!')
			b.WriteRune(r + ('a' - 'A'))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
