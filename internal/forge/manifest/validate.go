package manifest

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// segmentPattern matches a single valid Alloy component name segment.
var segmentPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// Validate checks that the manifest is semantically correct.
// It returns a descriptive error for the first validation failure found.
func (m *Manifest) Validate() error {
	if err := validateName(m.Name); err != nil {
		return err
	}

	switch m.Type {
	case ComponentTypeAlloy, ComponentTypeOtelReceiver, ComponentTypeOtelExporter:
		// valid
	case "":
		return errors.New("type is required")
	default:
		return fmt.Errorf("unknown type %q", m.Type)
	}

	switch m.Stability {
	case StabilityExperimental, StabilityPublicPreview, StabilityGA:
		// valid
	case "":
		return errors.New("stability is required")
	default:
		return fmt.Errorf("unknown stability %q", m.Stability)
	}

	if err := validateSource(m.Source); err != nil {
		return err
	}

	if m.Type == ComponentTypeOtelReceiver {
		if m.OtelReceiver == nil {
			return fmt.Errorf("otelcol.receiver block is required when type is %q", ComponentTypeOtelReceiver)
		}
		if m.OtelReceiver.Factory == "" {
			return errors.New("otelcol.receiver.factory is required")
		}
	}

	if m.OtelReceiver != nil && m.OtelReceiver.Config != nil {
		if err := validateConfigSchema(m.OtelReceiver.Config); err != nil {
			return err
		}
	}

	return nil
}

func validateName(name string) error {
	if name == "" {
		return errors.New("name is required")
	}
	for seg := range strings.SplitSeq(name, ".") {
		if seg == "" {
			return fmt.Errorf("invalid name %q: empty segment", name)
		}
		if !segmentPattern.MatchString(seg) {
			return fmt.Errorf("invalid name %q: segment %q must match [a-z][a-z0-9_]*", name, seg)
		}
	}
	return nil
}

func validateSource(src SourceConfig) error {
	hasImport := src.Import != ""
	hasDir := src.Dir != ""

	if !hasImport && !hasDir {
		return errors.New("source must have either import or dir set")
	}
	// When dir is set, import is optional but recommended (used for Yaegi import path).
	// When only import is set, it's used for remote module resolution.
	return nil
}

func validateConfigSchema(cfg *ConfigSchema) error {
	if cfg.Type == "" {
		return errors.New("config.type is required when a config block is present")
	}
	if err := validateRequired(cfg.Required, "config"); err != nil {
		return err
	}
	return nil
}

func validateRequired(required []string, context string) error {
	for i, r := range required {
		if strings.TrimSpace(r) == "" {
			return fmt.Errorf("%s.required[%d] must not be blank", context, i)
		}
	}
	return nil
}
