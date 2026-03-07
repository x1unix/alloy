// Package mapper translates Alloy River config blocks into OTel component config structs.
package mapper

import (
	"fmt"
	"strings"

	"github.com/grafana/alloy/internal/forge/manifest"
	"github.com/mitchellh/mapstructure"
)

// MapConfig decodes an Alloy arguments map into a config struct instance.
// args is a map[string]any decoded from the River config.
// schema is the manifest config schema.
// target is a pointer to the config struct to populate.
func MapConfig(args map[string]any, schema *manifest.ConfigSchema, target any) error {
	if err := validateRequired(args, schema.Required, ""); err != nil {
		return err
	}

	if err := validateProperties(args, schema.Properties, ""); err != nil {
		return err
	}

	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:           target,
		TagName:          "mapstructure",
		WeaklyTypedInput: true,
	})
	if err != nil {
		return fmt.Errorf("create decoder: %w", err)
	}

	if err := decoder.Decode(args); err != nil {
		return fmt.Errorf("decode config: %w", err)
	}
	return nil
}

func validateRequired(args map[string]any, required []string, prefix string) error {
	for _, name := range required {
		if _, ok := args[name]; !ok {
			field := name
			if prefix != "" {
				field = prefix + "." + name
			}
			return fmt.Errorf("required field %q is missing", field)
		}
	}
	return nil
}

func validateProperties(args map[string]any, props map[string]manifest.PropertySchema, prefix string) error {
	for name, val := range args {
		prop, ok := props[name]
		if !ok {
			continue
		}

		if prop.Type == manifest.PropertyTypeObject {
			sub, ok := val.(map[string]any)
			if !ok {
				continue
			}

			fieldPath := name
			if prefix != "" {
				fieldPath = prefix + "." + name
			}

			if err := validateRequired(sub, prop.Required, fieldPath); err != nil {
				return err
			}
			if err := validateProperties(sub, prop.Properties, fieldPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// RiverToMap converts an Alloy River config body into a map[string]any,
// handling block vs attribute distinctions from the schema.
// body is the raw River block body as decoded by the Alloy syntax package.
func RiverToMap(body map[string]any, props map[string]manifest.PropertySchema) map[string]any {
	result := make(map[string]any, len(body))
	for key, val := range body {
		prop, hasProp := props[key]

		if hasProp && prop.Type == manifest.PropertyTypeObject && prop.Block {
			// Nested block: val should be a map or []map.
			if sub, ok := val.(map[string]any); ok {
				result[key] = RiverToMap(sub, prop.Properties)
				continue
			}
		}

		result[key] = val
	}
	return result
}

// LookupType resolves a Go type string like "*pkg.Config" into its package and type name.
// Returns (packagePath, typeName, isPointer).
func LookupType(typeStr string) (pkgPath, typeName string, isPointer bool, err error) {
	s := typeStr
	if strings.HasPrefix(s, "*") {
		isPointer = true
		s = s[1:]
	}

	lastDot := strings.LastIndexByte(s, '.')
	if lastDot < 0 {
		return "", "", false, fmt.Errorf("type %q has no package qualifier", typeStr)
	}

	pkgPath = s[:lastDot]
	typeName = s[lastDot+1:]
	return pkgPath, typeName, isPointer, nil
}
