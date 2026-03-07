// Package otelreceiver provides the Forge shim for otelcol.receiver plugins.
package otelreceiver

import (
	"fmt"
	"reflect"

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

	return component.TryRegister(component.Registration{
		Name:      m.Name,
		Stability: convertStability(m.Stability),
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

	return &forgeReceiverArgs{
		config:       cfg,
		output:       output,
		debugMetrics: debugMetrics,
	}, nil
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
