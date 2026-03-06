# Alloy Forge

Alloy Forge is an experimental hackathon feature for Grafana Alloy that enables dynamic loading of custom components from Go source files at runtime, without rebuilding Alloy.

## Overview

Normally, Alloy components are compiled into the binary. Every new component requires modifying the Alloy source tree, implementing the `component.Component` interface, calling `component.Register`, and rebuilding the binary. This creates a high barrier for users who want custom pipeline logic.

Alloy Forge removes that barrier by using the [Yaegi](https://github.com/traefik/yaegi) Go interpreter to evaluate user-provided Go source files at runtime. A lightweight YAML manifest describes the component metadata and maps Alloy config arguments to the interpreted component's inputs, allowing the Alloy controller to treat the plugin as a first-class component.

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│ Alloy runtime                                            │
│                                                          │
│  ┌───────────────┐        ┌──────────────────────────┐  │
│  │ Alloy config  │──────▶ │ Forge loader             │  │
│  │ (.alloy file) │        │                          │  │
│  └───────────────┘        │  1. Read plugin manifest  │  │
│                           │  2. Interpret source via  │  │
│  ┌───────────────┐        │     Yaegi                 │  │
│  │ Plugin dir    │──────▶ │  3. Build argument mapper │  │
│  │               │        │  4. Call component.       │  │
│  │  forge.yaml   │        │     Register dynamically  │  │
│  │  component.go │        └──────────────────────────┘  │
│  └───────────────┘                                       │
└─────────────────────────────────────────────────────────┘
```

### Components

- **Forge loader** — discovers plugin directories, reads manifests, and invokes Yaegi to interpret Go source files.
- **Argument mapper** — translates Alloy `Arguments` (decoded from the `.alloy` config) into the plugin's input struct using reflection, guided by field mappings declared in the manifest.
- **Dynamic registrar** — wraps the interpreted component in a shim that satisfies `component.Component` and calls `component.Register` with a synthesised `component.Registration`.

## Plugin structure

Each Forge plugin lives in its own directory:

```
my-plugin/
├── forge.yaml       # Plugin manifest
└── component.go     # Go source interpreted by Yaegi
```

### Manifest (`forge.yaml`)

The manifest describes the component to Alloy and defines how Alloy config arguments map to the Go struct fields inside the plugin:

```yaml
# forge.yaml

# name is the Alloy component name, e.g. "forge.myplugin"
name: forge.myplugin

# stability mirrors featuregate.Stability values: experimental, public-preview, generally-available
stability: experimental

# description is shown in the Alloy UI and docs
description: "A custom Forge component that does X."

# args declares the arguments the component accepts and how they map
# to fields on the Go struct returned by the plugin's New() constructor.
args:
  - name: endpoint      # Alloy config field name
    type: string        # Alloy type used for config decoding
    field: Endpoint     # Exported Go struct field on the plugin's Args struct
    required: true

  - name: timeout
    type: duration
    field: Timeout
    default: "30s"

# exports declares values the component exposes to other Alloy components.
# Leave empty if the component has no exports.
exports:
  - name: receiver
    type: otelcol.Consumer
    field: Receiver
```

### Go source (`component.go`)

The plugin implements a small, well-defined interface that Forge expects to find via Yaegi:

```go
package myplugin

import (
    "context"
)

// Args holds the configuration values mapped from the Alloy config by Forge.
type Args struct {
    Endpoint string
    Timeout  time.Duration
}

// Component is the plugin implementation.
type Component struct {
    args Args
}

// New constructs the component. Forge calls this once per component instance.
func New(args Args) (*Component, error) {
    return &Component{args: args}, nil
}

// Run starts the component. Forge calls this in a goroutine and cancels ctx on shutdown.
func (c *Component) Run(ctx context.Context) error {
    // plugin logic here
    <-ctx.Done()
    return nil
}

// Update applies new arguments. Forge calls this whenever the Alloy config changes.
func (c *Component) Update(args Args) error {
    c.args = args
    return nil
}
```

## Argument type mapping

The manifest `type` field controls how the Alloy config value is decoded before it is set on the Go struct field:

| Manifest type | Go type              |
|---------------|----------------------|
| `string`      | `string`             |
| `int`         | `int`                |
| `float`       | `float64`            |
| `bool`        | `bool`               |
| `duration`    | `time.Duration`      |
| `list(T)`     | `[]T`                |
| `map(T)`      | `map[string]T`       |

Complex Alloy types (e.g. `otelcol.Consumer`) are passed through as opaque interface values and the plugin is responsible for asserting the correct type.

## Forge loader Alloy block

Forge plugins are loaded via a top-level `forge.plugin` block in the Alloy config:

```alloy
forge.plugin "myplugin" {
  path = "/etc/alloy/plugins/my-plugin"
}

forge.myplugin "example" {
  endpoint = "http://localhost:4317"
  timeout  = "10s"
}
```

The `forge.plugin` block instructs the Forge loader to read the manifest at the given path, interpret the Go source, and dynamically register `forge.myplugin` so it can be used like any built-in component.

## Constraints and limitations

- Yaegi does not support all Go language features. CGO, unsafe pointer arithmetic, and certain reflection patterns are unavailable inside plugins.
- Plugins run in the same process as Alloy. A panicking plugin will crash the process.
- The symbol table exposed to plugins is limited to the packages explicitly allowed by the Forge loader. Plugins cannot import arbitrary third-party packages unless they are pre-linked into the Alloy binary.
- `component.Register` is called at load time. Hot-reloading a plugin requires restarting Alloy.
- The Forge loader is experimental and not subject to Alloy's stability guarantees.

## Goals for the hackathon

- [ ] Implement the Forge loader that reads `forge.yaml` and starts Yaegi
- [ ] Implement the argument mapper (manifest types → Go struct fields via reflection)
- [ ] Implement the `component.Component` shim that wraps the interpreted plugin
- [ ] Wire the `forge.plugin` Alloy block into the controller
- [ ] Write an end-to-end example plugin (e.g. a simple log transformer)
- [ ] Document the allowed symbol table (which packages plugins may import)
