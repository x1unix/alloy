# Alloy Forge — Implementation Plan

Hackathon implementation plan. Tasks are ordered roughly by dependency.

---

## 1. Manifest parsing

**Package:** `internal/forge/manifest`

Define Go structs that map to `forge.yaml` and implement YAML unmarshaling.

- [ ] `Manifest` top-level struct (`name`, `stability`, `community`, `type`, `source`)
- [ ] `SourceConfig` struct (`import`, `version`, `packageName`, `dir`)
- [ ] `OtelReceiverConfig` struct (`factory`, `config`)
- [ ] `ConfigSchema` struct (`type`, `required`, `properties`)
- [ ] `PropertySchema` struct (`type`, `block`, `required`, `properties`) — recursive for nested objects
- [ ] Validation: `name` is a valid Alloy component name, `type` is a known value, required fields are present
- [ ] Unit tests for valid and invalid manifests

---

## 2. Yaegi interpreter setup

**Package:** `internal/forge/capsule`

Initialise and configure the Yaegi interpreter with the pre-exported symbol table.

- [ ] Create interpreter with `interp.New(interp.Options{...})`
- [ ] Export standard library symbols via `stdlib.Symbols`
- [ ] Export `unsafe` symbols
- [ ] Export `go.opentelemetry.io/collector/...` sub-packages (`component`, `consumer`, `receiver`, `exporter`, `pdata/plog`, `pdata/pmetric`, `pdata/ptrace`)
- [ ] Export `go.uber.org/zap`
- [ ] Export `github.com/mitchellh/mapstructure`
- [ ] Helper: `EvalFactory(src string) (any, error)` — wraps a factory snippet in a callable function body and returns the result

---

## 3. Package resolver

**Package:** `internal/forge/resolver`

Locate or download the Go package that a manifest's `source` block points to.

Forge does **not** require Go to be installed on the host. All module fetching is done directly against the Go module proxy API at `https://proxy.golang.org/` using plain HTTP — no `go` toolchain invocation.

### Go module proxy API

The proxy exposes a stable REST API that Forge calls directly:

| Endpoint                                  | Description                                     |
| ----------------------------------------- | ----------------------------------------------- |
| `GET $GOPROXY/<module>/@v/list`           | List available versions                         |
| `GET $GOPROXY/<module>/@latest`           | Resolve `latest` to a concrete version          |
| `GET $GOPROXY/<module>/@v/<version>.info` | Version metadata (timestamp, canonical version) |
| `GET $GOPROXY/<module>/@v/<version>.mod`  | `go.mod` for the version                        |
| `GET $GOPROXY/<module>/@v/<version>.zip`  | Module zip archive (standard `zip` layout)      |

The proxy URL defaults to `https://proxy.golang.org` but should be overridable via a `GOPROXY`-style env var for air-gapped environments.

### Task checklist

- [ ] If `source.dir` is set: resolve relative path, verify it exists, return it — no network call needed
- [ ] If `source.import` is set:
  - [ ] Resolve `source.version` (or `latest`) to a concrete version via `/@latest` or `/@v/list`
  - [ ] Check `LoadConfig.ModCacheDir` for an already-extracted copy; skip download if present
  - [ ] Fetch `/<version>.zip` from the proxy and extract into `ModCacheDir`
- [ ] Expose the resolved source directory to Yaegi (`interp.Options.GoPath` or `UseGoPath`)
- [ ] Infer `packageName` from the last path segment of `source.import` when not set explicitly
- [ ] Respect `GOPROXY` / `GONOSUMCHECK` env vars for private registries and air-gapped setups
- [ ] Unit tests: local dir resolution, proxy fetch with a mock HTTP server, cache hit avoids re-download

---

## 4. Config mapper

**Package:** `internal/forge/mapper`

Translate an Alloy River config block into the OTel component's config struct using the manifest schema. Plugins are not aware of Forge — they only see their own config type, as if constructed normally.

### River unmarshaling

The Alloy syntax package (`github.com/grafana/alloy/syntax`) is the canonical River config parser. It exposes:

- `syntax.Unmarshal(in []byte, v any) error` — decodes a River config file into a Go value
- `syntax.UnmarshalValue(in []byte, v any) error` — decodes a River expression into a Go value

`syntax.Unmarshal` natively decodes into `map[string]any` when the target is a map, treating each River attribute as a map key. This is the entry point for the config mapper.

### Mapping flow

1. **River → `map[string]any`**: call `syntax.Unmarshal` with a `map[string]any` target, guided by the manifest schema to handle nested `block: true` properties as sub-maps.
2. **Construct config instance in Yaegi**: use Yaegi's symbol table to look up the type named in `config.type` (e.g. `*awss3receiver.Config`) and create a zero-value instance of it via reflection. This is necessary because the type only exists inside the Yaegi interpreter — it must be the same instance the factory will later receive.
3. **`map[string]any` → config struct**: call `mapstructure.Decode(map, configInstance)` to populate the struct. OTel components already carry `mapstructure` struct tags, so no additional mapping logic is needed.
4. **Pass to factory**: the populated config value is passed as `any` to the factory's `Create*Receiver` (or equivalent) method, which casts it internally to its concrete type (e.g. `*awss3receiver.Config`).

### Task checklist

- [ ] Implement River → `map[string]any` decoding using `syntax.Unmarshal`, honouring `block: true` for nested blocks vs attributes
- [ ] Implement Yaegi type lookup: resolve the type string from `config.type` in the interpreter's symbol table and instantiate it with `reflect.New`
- [ ] Call `mapstructure.Decode` to populate the Yaegi-constructed config instance from the map
- [ ] Validate required fields (from manifest `required` lists) before passing the config to the factory
- [ ] Unit tests: scalar types, nested blocks, required field validation errors

---

## 5. `otelcol.receiver` shim

**Package:** `internal/forge/otelreceiver`

Wrap an OTel `receiver.Factory` and a mapped config into a `component.Component`.

- [ ] Call `EvalFactory` to obtain a `receiver.Factory` from the manifest snippet
- [ ] On `component.Build`: call `factory.CreateLogsReceiver` / `CreateMetricsReceiver` / `CreateTracesReceiver` based on the declared outputs in the Alloy block
- [ ] Implement `Run(ctx)`: start the receiver, block until ctx cancels, then shut it down
- [ ] Implement `Update(args)`: re-map the config and restart the receiver gracefully
- [ ] Wire OTel consumer outputs to Alloy's `otelcol.Consumer` exports

---

## 6. Forge loader (`internal/forge`)

**Package:** `internal/forge`

Implement the `Load(cfg LoadConfig) error` entry point that ties everything together.

`LoadConfig.ModCacheDir` is the single cache root for all downloaded Go packages. This covers both the plugin package itself (resolved from `source.import`) and any of its transitive dependencies fetched via the module proxy. Plugins with `source.dir` set are loaded directly from the local path and bypass the cache entirely.

- [ ] Scan `LoadConfig.PluginsDir` for `*.yml` / `*.yaml` files
- [ ] Parse each file with the manifest parser (step 1)
- [ ] Pass `LoadConfig.ModCacheDir` to the package resolver (step 3) for all `source.import`-based plugins; skip for `source.dir` plugins
- [ ] Set up a Yaegi interpreter instance (step 2) — one interpreter per plugin or shared
- [ ] Based on `manifest.type`, delegate to the appropriate shim (step 5)
- [ ] Call `component.TryRegister` with the resulting `component.Registration`
- [ ] Return a descriptive error (not a panic) if registration fails

---

## 7. Bootstrap

Forge must run before Alloy starts reading the component registry, so plugins are registered before the controller processes any `.alloy` config blocks that reference them. There is no `forge.plugin` Alloy block for the PoC — configuration comes entirely from env vars and defaults.

| Env var                | Default                  | Description                                               |
| ---------------------- | ------------------------ | --------------------------------------------------------- |
| `ALLOY_FORGE_DIR`      | `$PWD/plugins`           | Directory scanned for `*.yml` / `*.yaml` manifests        |
| `ALLOY_FORGE_MODCACHE` | `$ALLOY_FORGE_DIR/cache` | Module cache directory passed as `LoadConfig.ModCacheDir` |

- [ ] Implement `LoadConfigFromEnv() LoadConfig` — reads the env vars above and falls back to the defaults
- [ ] Call `forge.Load(LoadConfigFromEnv())` early in Alloy's startup sequence, before the component controller initialises
- [ ] If `ALLOY_FORGE_DIR` does not exist, skip loading silently (Forge is opt-in)
- [ ] Surface any load errors as a fatal startup error with a clear message indicating which manifest failed and why

---

## 8. End-to-end demo

- [ ] Verify `plugins/forge.source.awss3.yml` loads correctly end-to-end
- [ ] Write a minimal `.alloy` config that loads the plugin and pipes output to `loki.write` or `otelcol.exporter.otlp`
- [ ] Document any extra steps (module cache directory, permissions, etc.) in the README

---

## Deferred / out of scope for hackathon

- `otelcol.exporter` shim (same pattern as receiver, lower priority)
- `alloy` type (fully custom component written against Forge's own interface)
- Hot-reload without process restart
- Sandboxing / panic recovery for interpreted code
- Automatic symbol table generation from OTel module graph
