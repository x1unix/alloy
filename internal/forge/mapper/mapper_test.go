package mapper_test

import (
	"testing"

	"github.com/grafana/alloy/internal/forge/manifest"
	"github.com/grafana/alloy/internal/forge/mapper"
)

type TestConfig struct {
	Region  string `mapstructure:"region"`
	Bucket  string `mapstructure:"s3_bucket"`
	Timeout int64  `mapstructure:"timeout"`
}

func TestMapConfig_Scalars(t *testing.T) {
	schema := &manifest.ConfigSchema{
		Type: "*TestConfig",
		Properties: map[string]manifest.PropertySchema{
			"region":   {Type: manifest.PropertyTypeString},
			"s3_bucket": {Type: manifest.PropertyTypeString},
			"timeout":  {Type: manifest.PropertyTypeInt64},
		},
	}
	args := map[string]any{
		"region":   "us-east-1",
		"s3_bucket": "my-bucket",
		"timeout":  int64(30),
	}

	var cfg TestConfig
	if err := mapper.MapConfig(args, schema, &cfg); err != nil {
		t.Fatalf("MapConfig: %v", err)
	}
	if cfg.Region != "us-east-1" {
		t.Errorf("Region = %q", cfg.Region)
	}
	if cfg.Bucket != "my-bucket" {
		t.Errorf("Bucket = %q", cfg.Bucket)
	}
	if cfg.Timeout != 30 {
		t.Errorf("Timeout = %d", cfg.Timeout)
	}
}

type NestedConfig struct {
	Downloader DownloaderConfig `mapstructure:"s3downloader"`
}

type DownloaderConfig struct {
	Bucket string `mapstructure:"s3_bucket"`
	Prefix string `mapstructure:"s3_prefix"`
}

func TestMapConfig_NestedBlock(t *testing.T) {
	schema := &manifest.ConfigSchema{
		Type: "*NestedConfig",
		Properties: map[string]manifest.PropertySchema{
			"s3downloader": {
				Type:  manifest.PropertyTypeObject,
				Block: true,
				Properties: map[string]manifest.PropertySchema{
					"s3_bucket": {Type: manifest.PropertyTypeString},
					"s3_prefix": {Type: manifest.PropertyTypeString},
				},
			},
		},
	}
	args := map[string]any{
		"s3downloader": map[string]any{
			"s3_bucket": "logs-bucket",
			"s3_prefix": "logs/",
		},
	}

	var cfg NestedConfig
	if err := mapper.MapConfig(args, schema, &cfg); err != nil {
		t.Fatalf("MapConfig: %v", err)
	}
	if cfg.Downloader.Bucket != "logs-bucket" {
		t.Errorf("Downloader.Bucket = %q", cfg.Downloader.Bucket)
	}
	if cfg.Downloader.Prefix != "logs/" {
		t.Errorf("Downloader.Prefix = %q", cfg.Downloader.Prefix)
	}
}

func TestMapConfig_RequiredMissing(t *testing.T) {
	schema := &manifest.ConfigSchema{
		Type:     "*TestConfig",
		Required: []string{"region"},
		Properties: map[string]manifest.PropertySchema{
			"region": {Type: manifest.PropertyTypeString},
		},
	}
	args := map[string]any{}

	var cfg TestConfig
	if err := mapper.MapConfig(args, schema, &cfg); err == nil {
		t.Fatal("expected error for missing required field")
	}
}

func TestMapConfig_NestedRequiredMissing(t *testing.T) {
	schema := &manifest.ConfigSchema{
		Type: "*NestedConfig",
		Properties: map[string]manifest.PropertySchema{
			"s3downloader": {
				Type:     manifest.PropertyTypeObject,
				Block:    true,
				Required: []string{"s3_bucket"},
				Properties: map[string]manifest.PropertySchema{
					"s3_bucket": {Type: manifest.PropertyTypeString},
				},
			},
		},
	}
	args := map[string]any{
		"s3downloader": map[string]any{
			// s3_bucket is missing
		},
	}

	var cfg NestedConfig
	if err := mapper.MapConfig(args, schema, &cfg); err == nil {
		t.Fatal("expected error for missing nested required field")
	}
}

func TestLookupType(t *testing.T) {
	tests := []struct {
		input     string
		pkgPath   string
		typeName  string
		isPointer bool
		wantErr   bool
	}{
		{"*awss3receiver.Config", "awss3receiver", "Config", true, false},
		{"mypackage.MyType", "mypackage", "MyType", false, false},
		{"github.com/org/repo/pkg.Type", "github.com/org/repo/pkg", "Type", false, false},
		{"NoPackage", "", "", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			pkg, name, ptr, err := mapper.LookupType(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("LookupType: %v", err)
			}
			if pkg != tt.pkgPath {
				t.Errorf("pkgPath = %q, want %q", pkg, tt.pkgPath)
			}
			if name != tt.typeName {
				t.Errorf("typeName = %q, want %q", name, tt.typeName)
			}
			if ptr != tt.isPointer {
				t.Errorf("isPointer = %v, want %v", ptr, tt.isPointer)
			}
		})
	}
}
