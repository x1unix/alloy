package forge

type LoadConfig struct {
	// PluginsDir is a path to load plugins from.
	PluginsDir string

	// ModCacheDir is a path to cache Go module dependencies.
	ModCacheDir string
}

// Load loads and registers dynamic components.
func Load(cfg LoadConfig) error {
	return nil
}
