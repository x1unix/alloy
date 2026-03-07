// Package otelreceiver provides the Forge shim for otelcol.receiver plugins.
package otelreceiver

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol/receiver"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/forge/manifest"
	"github.com/grafana/alloy/internal/forge/mapper"
	"github.com/grafana/alloy/internal/featuregate"
	otelreceiver "go.opentelemetry.io/collector/receiver"
)

// Register registers a forge otelcol.receiver component with the Alloy component registry.
func Register(m *manifest.Manifest, factory otelreceiver.Factory) error {
	schema := m.OtelReceiver.Config
	argsType := buildArgsType(schema)
	zeroArgs := reflect.New(argsType).Elem().Interface()

	stability := convertStability(m.Stability)
	if m.Community {
		stability = featuregate.StabilityUndefined
	}

	return component.TryRegister(component.Registration{
		Name:      m.Name,
		Stability: stability,
		Community: m.Community,
		Args:      zeroArgs,

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			rv := reflect.ValueOf(args)
			fargs, err := convertArgs(rv, schema, factory)
			if err != nil {
				return nil, fmt.Errorf("convert args: %w", err)
			}
			return receiver.New(opts, factory, fargs)
		},
	})
}

// convertArgs extracts config from a dynamically-typed args struct and builds receiver.Arguments.
func convertArgs(rv reflect.Value, schema *manifest.ConfigSchema, factory otelreceiver.Factory) (receiver.Arguments, error) {
	// Extract the output consumers.
	outputField := rv.FieldByName("ForgeOutput")
	var output *otelcol.ConsumerArguments
	if outputField.IsValid() && !outputField.IsNil() {
		output = outputField.Interface().(*otelcol.ConsumerArguments)
	}

	// Extract debug metrics config.
	debugField := rv.FieldByName("ForgeDebugMetrics")
	var debugMetrics otelcolCfg.DebugMetricsArguments
	if debugField.IsValid() {
		debugMetrics = debugField.Interface().(otelcolCfg.DebugMetricsArguments)
	}

	// Extract config fields into a map.
	configMap := extractConfigMap(rv, schema.Properties)

	// Create a default config from the factory and populate it with mapstructure.
	cfg := factory.CreateDefaultConfig()
	if len(configMap) > 0 {
		if err := mapper.MapConfig(configMap, schema, cfg); err != nil {
			return nil, fmt.Errorf("map config: %w", err)
		}
	}

	// Yaegi-interpreted configs don't preserve methods, so Validate()
	// can't be called. Apply common OTel defaults for zero-valued fields
	// that would normally be set by Validate().
	applyConfigDefaults(cfg)

	return &forgeReceiverArgs{
		config:       cfg,
		output:       output,
		debugMetrics: debugMetrics,
	}, nil
}

// applyConfigDefaults uses reflection to set reasonable defaults for
// zero-valued fields in Yaegi-interpreted config structs. Yaegi-interpreted
// types don't preserve methods, so we can't call the config's Validate()
// which normally sets these defaults.
func applyConfigDefaults(cfg any) {
	v := reflect.ValueOf(cfg)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}
	applyDefaultsRecursive(v)
}

func applyDefaultsRecursive(v reflect.Value) {
	for i := range v.NumField() {
		field := v.Field(i)
		if !field.CanSet() {
			continue
		}

		tag := v.Type().Field(i).Tag.Get("mapstructure")

		switch field.Kind() {
		case reflect.Struct:
			applyDefaultsRecursive(field)
		case reflect.Ptr:
			if !field.IsNil() && field.Elem().Kind() == reflect.Struct {
				applyDefaultsRecursive(field.Elem())
			}
		case reflect.Int64:
			// Common OTel pattern: max_request_body_size defaults to 20MB.
			if field.Int() == 0 && strings.Contains(tag, "max_request_body_size") {
				field.SetInt(20 * 1024 * 1024)
			}
		}
	}
}

func convertStability(s manifest.Stability) featuregate.Stability {
	switch s {
	case manifest.StabilityGA:
		return featuregate.StabilityGenerallyAvailable
	case manifest.StabilityPublicPreview:
		return featuregate.StabilityPublicPreview
	default:
		return featuregate.StabilityExperimental
	}
}
