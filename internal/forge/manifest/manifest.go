package manifest

// Manifest is the top-level structure of a forge plugin manifest file.
type Manifest struct {
	Name      string        `yaml:"name"`
	Stability Stability     `yaml:"stability"`
	Community bool          `yaml:"community"`
	Type      ComponentType `yaml:"type"`
	Source    SourceConfig  `yaml:"source"`

	// OtelReceiver holds configuration specific to otelcol.receiver plugins.
	// The YAML key contains a dot, which yaml.v3 handles correctly via the struct tag.
	OtelReceiver *OtelReceiverConfig `yaml:"otelcol.receiver"`
}

// SourceConfig describes where the plugin's Go package comes from.
type SourceConfig struct {
	// Import is the Go module import path (e.g. github.com/org/repo/receiver/myreceiver).
	Import string `yaml:"import"`

	// Version is the Go module version to fetch. Defaults to "latest" when empty.
	Version string `yaml:"version"`

	// PackageName overrides the package name inferred from the last segment of Import.
	PackageName string `yaml:"packageName"`

	// Dir is a local filesystem path to the package source, used for development.
	// Mutually exclusive with Import.
	Dir string `yaml:"dir"`
}

// OtelReceiverConfig holds OpenTelemetry receiver-specific configuration.
type OtelReceiverConfig struct {
	// Factory is a Go code snippet that returns a receiver.Factory.
	Factory string `yaml:"factory"`

	// Config describes the schema for the component's config block.
	Config *ConfigSchema `yaml:"config"`
}

// ConfigSchema describes the Alloy config block accepted by the component.
type ConfigSchema struct {
	// Type is the fully-qualified Go type string for the config struct
	// (e.g. "*awss3receiver.Config").
	Type string `yaml:"type"`

	// Required lists the names of properties that must be present.
	Required []string `yaml:"required"`

	// Properties maps each field name to its schema.
	Properties map[string]PropertySchema `yaml:"properties"`
}

// PropertySchema describes a single field within a ConfigSchema.
// The type is recursive: object properties can themselves have Properties.
type PropertySchema struct {
	// Type is the value type (string, bool, int64, float64, object).
	Type PropertyType `yaml:"type"`

	// Block indicates that this object property maps to an Alloy River block
	// rather than a plain attribute.
	Block bool `yaml:"block"`

	// Required lists required child property names (only meaningful for objects).
	Required []string `yaml:"required"`

	// Properties holds the schema for nested fields (only meaningful for objects).
	Properties map[string]PropertySchema `yaml:"properties"`
}
