# Alloy Forge

Alloy Forge is an experimental hackathon feature for Grafana Alloy that enables dynamic loading of components from existing Go packages at runtime, without rebuilding Alloy.

## Overview

Normally, adding a new Alloy component requires modifying the Alloy source tree, implementing the `component.Component` interface, calling `component.Register`, and rebuilding the binary. This is a high barrier for users who want to integrate existing upstream components — especially OpenTelemetry Collector receivers and exporters — that Alloy doesn't yet ship.

Alloy Forge removes that barrier. A YAML manifest describes how to load a Go package (from a module registry or a local directory), how to instantiate the component, and how Alloy config arguments map to the component's config struct. The Forge runtime uses the [Yaegi](https://github.com/traefik/yaegi) Go interpreter to evaluate package code at runtime and register the resulting component with the Alloy controller.

## Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│ Alloy runtime                                                     │
│                                                                   │
│  ┌─────────────────┐        ┌────────────────────────────────┐   │
│  │ forge.plugin    │──────▶ │ Forge loader                   │   │
│  │ Alloy block     │        │                                │   │
│  └─────────────────┘        │  1. Read forge.yaml manifest   │   │
│                             │  2. Download / locate package  │   │
│  ┌─────────────────┐        │  3. Evaluate factory snippet   │   │
│  │ forge.yaml      │──────▶ │     via Yaegi                  │   │
│  │ manifest        │        │  4. Build config mapper        │   │
│  └─────────────────┘        │  5. Call component.Register    │   │
│                             └────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────────┘
```

## Plugin manifest (`forge.yaml`)

Each Forge plugin is a YAML manifest file. No custom Go source is required — the manifest points to an existing Go package.

```yaml
# Human-readable Alloy component name, e.g. "forge.source.awss3".
name: forge.source.awss3

# stability mirrors featuregate.Stability values:
#   experimental, public-preview, generally-available
stability: experimental

# community marks the component as a community component.
community: true

# type controls the integration pattern.
# Supported values: "alloy", "otelcol.receiver", "otelcol.exporter"
type: otelcol.receiver

# source locates the Go package to load.
source:
  # Go module import path.
  import: github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver

  # Go module version. Omit to use the latest available version.
  # version: v0.120.0

  # Package name, if it differs from the last segment of the import path.
  # packageName: awss3receiver

  # Load from a local directory instead of a module registry.
  # Useful during development.
  # dir: ../my-receiver
```

### OTel receiver integration (`otelcol.receiver`)

When `type` is `otelcol.receiver`, the manifest includes an `otelcol.receiver` block that tells Forge how to obtain the receiver factory and how to build its config from the Alloy config:

```yaml
otelcol.receiver:
  # factory is a Go code snippet evaluated by Yaegi.
  # The source package is automatically imported using source.import.
  # The snippet must return a receiver.Factory.
  factory: |
    return awss3receiver.NewFactory()

  # config describes the receiver's configuration schema.
  config:
    # Destination Go type for config unmarshaling.
    # Forge converts the Alloy River config to a map, then uses
    # mapstructure to populate this type.
    type: "*awss3receiver.Config"

    required:
      - s3downloader

    properties:
      starttime:
        type: int64

      endtime:
        type: int64

      s3downloader:
        type: object
        block: true     # exposed as a nested River block in Alloy config
        required:
          - s3_bucket
          - s3_prefix
        properties:
          region:
            type: string
          s3_bucket:
            type: string
          s3_prefix:
            type: string
          s3_partition_format:
            type: string
          s3_partition_timezone:
            type: string
          file_prefix:
            type: string
          file_prefix_include_telemetry_type:
            type: bool
          endpoint:
            type: string
          endpoint_partition_id:
            type: string
          s3_force_path_style:
            type: bool

        sqs:
          type: object
          properties:
            queue_url:
              type: string
            region:
              type: string
            endpoint:
              type: string
            wait_time_seconds:
              type: int64
            max_number_of_messages:
              type: int64
```

## Config mapping

Forge uses a two-step process to translate Alloy config into the component's Go config struct:

1. **River → map** — the Alloy River config block is decoded into a `map[string]any` using the property schema defined in the manifest.
2. **map → struct** — [`mapstructure`](https://github.com/mitchellh/mapstructure) populates the target Go struct (specified by `config.type`) from the map. Most OTel Collector components already carry `mapstructure` tags, so no extra code is needed.

### Schema property fields

| Field      | Description |
|------------|-------------|
| `type`     | Scalar type (`string`, `bool`, `int64`, `float64`) or `object` for nested structs |
| `block`    | When `true`, the property is exposed as a nested River block in the Alloy config rather than an attribute |
| `required` | List of property names that must be present |
| `properties` | Nested property definitions (valid when `type: object`) |

## Using a Forge plugin in Alloy config

Once a manifest is in place, load it with the `forge.plugin` block, then use the registered component like any built-in component:

```alloy
forge.plugin "awss3" {
  path = "/etc/alloy/plugins/forge.source.awss3.yml"
}

forge.source.awss3 "prod_logs" {
  s3downloader {
    s3_bucket = "my-log-bucket"
    s3_prefix = "logs/"
    region    = "us-east-1"
  }

  output {
    logs = [loki.write.default.receiver]
  }
}
```

The `forge.plugin` block instructs the Forge loader to read the manifest, download or locate the package, evaluate the factory snippet via Yaegi, and call `component.TryRegister` so the component is available to the controller.

## Pre-exported symbol table

Yaegi supports extending the interpreter's symbol table with additional packages and symbols pre-linked from the host binary. Forge uses this to make a curated set of packages available to factory snippets and interpreted code without requiring them to be downloaded separately.

The following packages are pre-exported by the Forge loader:

| Package | Purpose |
|---------|---------|
| `unsafe` | Required by some OTel components for low-level memory operations |
| `go.opentelemetry.io/collector/...` | Core OTel Collector types: `component`, `consumer`, `receiver`, `exporter`, `pdata`, etc. |
| `go.uber.org/zap` | Structured logging, used widely across OTel Collector components |
| `github.com/mitchellh/mapstructure` | Used internally by the config mapper and available to plugins that need to decode nested structures manually |

Packages outside this list are not available to Yaegi unless they are explicitly added to the symbol table. This is intentional: limiting the available surface area reduces the risk of interpreted code calling into host internals unexpectedly.

## Constraints and limitations

- Yaegi does not support all Go language features. CGO and some reflection patterns are unavailable inside evaluated code.
- Plugins run in the same process as Alloy. A panic in an interpreted factory snippet or receiver will crash the process.
- Plugins can only import packages from the pre-exported symbol table or from the standard library. Arbitrary third-party packages are not available unless added to the symbol table and pre-linked into the Alloy binary.
- `component.TryRegister` is called at load time. Hot-reloading a plugin requires restarting Alloy.
- The Forge loader is experimental and not subject to Alloy's stability guarantees.

## Hackathon goals

- [ ] Implement the Forge loader: read manifest, resolve package source, run Yaegi
- [ ] Implement the config mapper: River → map → mapstructure → Go struct
- [ ] Implement the `otelcol.receiver` shim: wrap the factory and config into a `component.Component`
- [ ] Wire the `forge.plugin` Alloy block into the controller
- [ ] End-to-end demo: load `forge.source.awss3` from `plugins/forge.source.awss3.yml`
- [ ] Document the allowlisted symbol table (packages available to Yaegi)
