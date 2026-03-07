package otelreceiver

import (
	"fmt"
	"reflect"
	"strings"
	"unicode"

	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/forge/manifest"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
)

var (
	consumerArgsType  = reflect.TypeOf((*otelcol.ConsumerArguments)(nil))
	debugMetricsType  = reflect.TypeOf(otelcolCfg.DebugMetricsArguments{})
	mapStringAnyType  = reflect.TypeOf(map[string]any{})
	stringType        = reflect.TypeOf("")
	boolType          = reflect.TypeOf(false)
	int64Type         = reflect.TypeOf(int64(0))
	float64Type       = reflect.TypeOf(float64(0))
)

// buildArgsType creates a dynamic struct type from the manifest config schema.
// The struct has alloy tags for each config property, plus Output and DebugMetrics fields.
func buildArgsType(schema *manifest.ConfigSchema) reflect.Type {
	var fields []reflect.StructField

	if schema != nil {
		for name, prop := range schema.Properties {
			ft, tag := fieldForProp(name, prop)
			fields = append(fields, reflect.StructField{
				Name: exportName(name),
				Type: ft,
				Tag:  reflect.StructTag(tag),
			})
		}
	}

	// Output block — required, wired to downstream consumers.
	fields = append(fields, reflect.StructField{
		Name: "ForgeOutput",
		Type: consumerArgsType,
		Tag:  `alloy:"output,block"`,
	})

	// Debug metrics — optional.
	fields = append(fields, reflect.StructField{
		Name: "ForgeDebugMetrics",
		Type: debugMetricsType,
		Tag:  `alloy:"debug_metrics,block,optional"`,
	})

	return reflect.StructOf(fields)
}

func fieldForProp(name string, prop manifest.PropertySchema) (reflect.Type, string) {
	if prop.Type == manifest.PropertyTypeObject && prop.Block {
		inner := buildBlockType(prop.Properties)
		return inner, fmt.Sprintf(`alloy:"%s,block,optional"`, name)
	}
	if prop.Type == manifest.PropertyTypeObject {
		return mapStringAnyType, fmt.Sprintf(`alloy:"%s,attr,optional"`, name)
	}
	return goType(prop.Type), fmt.Sprintf(`alloy:"%s,attr,optional"`, name)
}

// buildBlockType recursively creates a struct type for a block with nested properties.
func buildBlockType(props map[string]manifest.PropertySchema) reflect.Type {
	var fields []reflect.StructField
	for name, prop := range props {
		ft, tag := fieldForProp(name, prop)
		fields = append(fields, reflect.StructField{
			Name: exportName(name),
			Type: ft,
			Tag:  reflect.StructTag(tag),
		})
	}
	if len(fields) == 0 {
		return mapStringAnyType
	}
	return reflect.StructOf(fields)
}

func goType(pt manifest.PropertyType) reflect.Type {
	switch pt {
	case manifest.PropertyTypeBool:
		return boolType
	case manifest.PropertyTypeInt64:
		return int64Type
	case manifest.PropertyTypeFloat64:
		return float64Type
	default:
		return stringType
	}
}

// exportName converts a snake_case config key to an exported Go identifier.
func exportName(name string) string {
	parts := strings.Split(name, "_")
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteRune(unicode.ToUpper(rune(p[0])))
		b.WriteString(p[1:])
	}
	return b.String()
}

// extractConfigMap extracts config values from a decoded dynamic struct into a map.
func extractConfigMap(v reflect.Value, props map[string]manifest.PropertySchema) map[string]any {
	result := make(map[string]any, len(props))
	for name, prop := range props {
		field := v.FieldByName(exportName(name))
		if !field.IsValid() || field.IsZero() {
			continue
		}
		if prop.Type == manifest.PropertyTypeObject && prop.Block {
			result[name] = extractConfigMap(field, prop.Properties)
		} else {
			result[name] = field.Interface()
		}
	}
	return result
}

// forgeReceiverArgs implements receiver.Arguments for a dynamically loaded plugin.
type forgeReceiverArgs struct {
	config       otelcomponent.Config
	output       *otelcol.ConsumerArguments
	debugMetrics otelcolCfg.DebugMetricsArguments
}

func (a *forgeReceiverArgs) Convert() (otelcomponent.Config, error) {
	return a.config, nil
}

func (a *forgeReceiverArgs) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

func (a *forgeReceiverArgs) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

func (a *forgeReceiverArgs) NextConsumers() *otelcol.ConsumerArguments {
	return a.output
}

func (a *forgeReceiverArgs) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return a.debugMetrics
}
