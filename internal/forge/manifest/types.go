package manifest

// ComponentType identifies what kind of component a plugin provides.
type ComponentType string

const (
	ComponentTypeAlloy        ComponentType = "alloy"
	ComponentTypeOtelReceiver ComponentType = "otelcol.receiver"
	ComponentTypeOtelExporter ComponentType = "otelcol.exporter"
)

// Stability indicates the maturity level of a component.
type Stability string

const (
	StabilityExperimental  Stability = "experimental"
	StabilityPublicPreview Stability = "public-preview"
	StabilityGA            Stability = "ga"
)

// PropertyType identifies the type of a config property.
type PropertyType string

const (
	PropertyTypeString  PropertyType = "string"
	PropertyTypeBool    PropertyType = "bool"
	PropertyTypeInt64   PropertyType = "int64"
	PropertyTypeFloat64 PropertyType = "float64"
	PropertyTypeObject  PropertyType = "object"
)
