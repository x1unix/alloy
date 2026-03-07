package forge

import (
	"os"
	"path/filepath"
)

const (
	envForgeDir      = "ALLOY_FORGE_DIR"
	envForgeModCache = "ALLOY_FORGE_MODCACHE"
)

// LoadConfigFromEnv reads forge configuration from environment variables,
// falling back to sensible defaults.
func LoadConfigFromEnv() LoadConfig {
	pluginsDir := os.Getenv(envForgeDir)
	if pluginsDir == "" {
		pluginsDir = filepath.Join(".", "plugins")
	}

	modCache := os.Getenv(envForgeModCache)
	if modCache == "" {
		modCache = filepath.Join(pluginsDir, "cache")
	}

	return LoadConfig{
		PluginsDir:  pluginsDir,
		ModCacheDir: modCache,
	}
}
