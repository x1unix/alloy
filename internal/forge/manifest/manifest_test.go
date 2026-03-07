package manifest_test

import (
	"os"
	"strings"
	"testing"

	"github.com/grafana/alloy/internal/forge/manifest"
)

const minimalValidYAML = `
name: forge.test.receiver
stability: experimental
type: otelcol.receiver
source:
  import: github.com/example/myreceiver
otelcol.receiver:
  factory: |
    return myreceiver.NewFactory()
  config:
    type: "*myreceiver.Config"
`

func TestParse_AwsS3Manifest(t *testing.T) {
	f, err := os.Open("testdata/forge.source.awss3.yml")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	m, err := manifest.Parse(f)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if m.Name != "forge.source.awss3" {
		t.Errorf("Name = %q, want %q", m.Name, "forge.source.awss3")
	}
	if m.Stability != manifest.StabilityExperimental {
		t.Errorf("Stability = %q, want %q", m.Stability, manifest.StabilityExperimental)
	}
	if m.Type != manifest.ComponentTypeOtelReceiver {
		t.Errorf("Type = %q, want %q", m.Type, manifest.ComponentTypeOtelReceiver)
	}
	if m.Source.Import != "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver" {
		t.Errorf("Source.Import = %q", m.Source.Import)
	}
	if m.OtelReceiver == nil {
		t.Fatal("OtelReceiver is nil")
	}
	if !strings.Contains(m.OtelReceiver.Factory, "awss3receiver.NewFactory()") {
		t.Errorf("Factory snippet does not contain expected call: %q", m.OtelReceiver.Factory)
	}
	if m.OtelReceiver.Config == nil {
		t.Fatal("OtelReceiver.Config is nil")
	}
	if m.OtelReceiver.Config.Type != "*awss3receiver.Config" {
		t.Errorf("Config.Type = %q", m.OtelReceiver.Config.Type)
	}

	s3dl, ok := m.OtelReceiver.Config.Properties["s3downloader"]
	if !ok {
		t.Fatal("s3downloader property not found")
	}
	if s3dl.Type != manifest.PropertyTypeObject {
		t.Errorf("s3downloader.Type = %q, want object", s3dl.Type)
	}
	if !s3dl.Block {
		t.Error("s3downloader.Block should be true")
	}
	if len(s3dl.Required) == 0 {
		t.Error("s3downloader.Required should not be empty")
	}
	if _, ok := s3dl.Properties["s3_bucket"]; !ok {
		t.Error("s3downloader.Properties missing s3_bucket")
	}
}

func TestValidate_MinimalValid(t *testing.T) {
	m, err := manifest.Parse(strings.NewReader(minimalValidYAML))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := m.Validate(); err != nil {
		t.Errorf("Validate() unexpected error: %v", err)
	}
}

func TestValidate_InvalidNames(t *testing.T) {
	names := []string{
		"",
		"Forge.Source",
		"forge..awss3",
		"forge.source.awss3!",
		"forge.source.1awss3",
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			m, err := manifest.Parse(strings.NewReader(minimalValidYAML))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			m.Name = name
			if err := m.Validate(); err == nil {
				t.Errorf("Validate() with name %q: expected error, got nil", name)
			}
		})
	}
}

func TestValidate_UnknownType(t *testing.T) {
	m, err := manifest.Parse(strings.NewReader(minimalValidYAML))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	m.Type = "otelcol.processor"
	if err := m.Validate(); err == nil {
		t.Error("Validate() with unknown type: expected error, got nil")
	}
}

func TestValidate_UnknownStability(t *testing.T) {
	m, err := manifest.Parse(strings.NewReader(minimalValidYAML))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	m.Stability = "beta"
	if err := m.Validate(); err == nil {
		t.Error("Validate() with unknown stability: expected error, got nil")
	}
}

func TestValidate_MissingSource(t *testing.T) {
	yaml := `
name: forge.test.receiver
stability: experimental
type: otelcol.receiver
source: {}
otelcol.receiver:
  factory: return myreceiver.NewFactory()
`
	m, err := manifest.Parse(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := m.Validate(); err == nil {
		t.Error("Validate() with missing source: expected error, got nil")
	}
}

func TestValidate_ConflictingSource(t *testing.T) {
	yaml := `
name: forge.test.receiver
stability: experimental
type: otelcol.receiver
source:
  import: github.com/example/myreceiver
  dir: ../myreceiver
otelcol.receiver:
  factory: return myreceiver.NewFactory()
`
	m, err := manifest.Parse(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := m.Validate(); err == nil {
		t.Error("Validate() with both import and dir: expected error, got nil")
	}
}

func TestValidate_MissingFactory(t *testing.T) {
	yaml := `
name: forge.test.receiver
stability: experimental
type: otelcol.receiver
source:
  import: github.com/example/myreceiver
otelcol.receiver:
  factory: ""
`
	m, err := manifest.Parse(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := m.Validate(); err == nil {
		t.Error("Validate() with empty factory: expected error, got nil")
	}
}

func TestValidate_MissingOtelReceiverBlock(t *testing.T) {
	yaml := `
name: forge.test.receiver
stability: experimental
type: otelcol.receiver
source:
  import: github.com/example/myreceiver
`
	m, err := manifest.Parse(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := m.Validate(); err == nil {
		t.Error("Validate() with missing otelcol.receiver block: expected error, got nil")
	}
}
