# Alloy Forge — Progress

See [PLAN.md](./PLAN.md) for the full implementation plan.

## Step 1: Manifest parsing (`internal/forge/manifest`)

### T1.1 — Define core manifest structs

Create `internal/forge/manifest/manifest.go` with the following Go structs matching the YAML schema:

- [x] `Manifest` — top-level: `Name`, `Stability`, `Community`, `Type`, `Source`
- [x] `SourceConfig` — `Import`, `Version`, `PackageName`, `Dir`
- [x] `OtelReceiverConfig` — `Factory`, `Config`
- [x] `ConfigSchema` — `Type`, `Required`, `Properties`
- [x] `PropertySchema` — `Type`, `Block`, `Required`, `Properties` (recursive, `map[string]PropertySchema`)

Use `yaml` struct tags throughout. `Properties` on `ConfigSchema` and `PropertySchema` should be `map[string]PropertySchema`.

### T1.2 — Define known enum values and constants

In the same package (or a `types.go` file), define:

- [x] `ComponentType` string type with constants: `ComponentTypeAlloy`, `ComponentTypeOtelReceiver`, `ComponentTypeOtelExporter`
- [x] `Stability` string type with constants: `StabilityExperimental`, `StabilityPublicPreview`, `StabilityGA`
- [x] `PropertyType` string type with constants for `string`, `bool`, `int64`, `float64`, `object`

### T1.3 — Implement YAML unmarshaling

Add a `Parse(r io.Reader) (*Manifest, error)` function in `internal/forge/manifest/parse.go` that:

- [x] Decodes YAML from the reader into `Manifest`
- [x] Handles the `otelcol.receiver` YAML key (dot in key name requires special handling — either a custom `UnmarshalYAML` or a raw intermediate struct)

### T1.4 — Implement validation

Add `func (m *Manifest) Validate() error` that checks:

- [x] `name` is non-empty and is a valid Alloy component name (dot-separated segments, each matching `[a-z][a-z0-9_]*`)
- [x] `type` is one of the known `ComponentType` constants
- [x] `stability` is one of the known `Stability` constants
- [x] `source` has exactly one of `import` or `dir` set (not both, not neither)
- [x] If `type` is `otelcol.receiver`, the `otelcol.receiver` block is present and `factory` is non-empty
- [x] `config.type` is non-empty when a config block is present
- [x] Required fields lists are non-empty slices of strings (no blank entries)

Return a descriptive `error` (not a panic) for each failure case.

### T1.5 — Unit tests

Create `internal/forge/manifest/manifest_test.go` covering:

- [x] **Happy path**: parse `plugins/forge.source.awss3.yml` and assert all fields decode correctly, including nested `s3downloader` properties
- [x] **Valid manifest**: a minimal valid `otelcol.receiver` manifest passes `Validate()`
- [x] **Invalid `name`**: names like `""`, `"Forge.Source"`, `"forge..awss3"`, `"forge.source.awss3!"` are rejected
- [x] **Unknown `type`**: `type: otelcol.processor` is rejected
- [x] **Unknown `stability`**: `stability: beta` is rejected
- [x] **Missing `source`**: both `import` and `dir` absent is rejected
- [x] **Conflicting `source`**: both `import` and `dir` set is rejected
- [x] **Missing factory**: `otelcol.receiver` block present but `factory` empty is rejected
- [x] **Missing required block**: `type: otelcol.receiver` but no `otelcol.receiver` block is rejected

---

## Step 2: Yaegi interpreter setup (`internal/forge/capsule`)

- [x] Create interpreter with `interp.New(interp.Options{...})`
- [x] Export standard library symbols via `stdlib.Symbols`
- [x] Export `unsafe` symbols
- [x] Export `go.opentelemetry.io/collector/...` sub-packages (`component`, `consumer`, `receiver`, `exporter`, `pdata/plog`, `pdata/pmetric`, `pdata/ptrace`)
- [x] Export `go.uber.org/zap`
- [x] Export `github.com/mitchellh/mapstructure`
- [x] Helper: `EvalFactory(src string) (any, error)` — wraps a factory snippet in a callable function body and returns the result

---

## Step 3: Package resolver (`internal/forge/resolver`)

- [x] If `source.dir` is set: resolve relative path, verify it exists, return it
- [x] If `source.import` is set:
  - [x] Resolve `source.version` (or `latest`) to a concrete version via `/@latest` or `/@v/list`
  - [x] Check `LoadConfig.ModCacheDir` for an already-extracted copy; skip download if present
  - [x] Fetch `/<version>.zip` from the proxy and extract into `ModCacheDir`
- [x] Expose the resolved source directory to Yaegi (`interp.Options.GoPath` or `UseGoPath`)
- [x] Infer `packageName` from the last path segment of `source.import` when not set explicitly
- [x] Respect `GOPROXY` / `GONOSUMCHECK` env vars for private registries and air-gapped setups
- [x] Unit tests: local dir resolution, proxy fetch with a mock HTTP server, cache hit avoids re-download

---

## Step 4: Config mapper (`internal/forge/mapper`)

- [x] Implement River → `map[string]any` decoding using `syntax.Unmarshal`, honouring `block: true` for nested blocks vs attributes
- [ ] Implement Yaegi type lookup: resolve the type string from `config.type` in the interpreter's symbol table and instantiate it with `reflect.New`
- [x] Call `mapstructure.Decode` to populate the Yaegi-constructed config instance from the map
- [x] Validate required fields (from manifest `required` lists) before passing the config to the factory
- [x] Unit tests: scalar types, nested blocks, required field validation errors

---

## Step 5: `otelcol.receiver` shim (`internal/forge/otelreceiver`)

- [x] Call `EvalFactory` to obtain a `receiver.Factory` from the manifest snippet
- [x] On `component.Build`: delegates to existing `receiver.New` which handles signal-specific receiver creation
- [x] Implement `Run(ctx)`: delegates to existing `receiver.Receiver` via `receiver.New`
- [x] Implement `Update(args)`: re-maps config and delegates to existing `receiver.Receiver`
- [x] Wire OTel consumer outputs to Alloy's `otelcol.Consumer` exports via `forgeReceiverArgs`

---

## Step 6: Forge loader (`internal/forge`)

- [x] Scan `LoadConfig.PluginsDir` for `*.yml` / `*.yaml` files
- [x] Parse each file with the manifest parser (step 1)
- [x] Pass `LoadConfig.ModCacheDir` to the package resolver (step 3) for all `source.import`-based plugins; skip for `source.dir` plugins
- [x] Set up a Yaegi interpreter instance (step 2) — one interpreter per plugin
- [x] Based on `manifest.type`, delegate to the appropriate shim (step 5)
- [x] Call `component.TryRegister` with the resulting `component.Registration`
- [x] Return a descriptive error (not a panic) if registration fails

---

## Step 7: Bootstrap

- [x] Implement `LoadConfigFromEnv() LoadConfig` — reads env vars and falls back to defaults
- [x] Call `forge.Load(LoadConfigFromEnv())` early in Alloy's startup sequence, before the component controller initialises
- [x] If `ALLOY_FORGE_DIR` does not exist, skip loading silently (Forge is opt-in)
- [x] Surface any load errors as a fatal startup error with a clear message

---

## Step 8: End-to-end demo

- [ ] Verify `plugins/forge.source.awss3.yml` loads correctly end-to-end
- [ ] Write a minimal `.alloy` config that loads the plugin and pipes output to `loki.write` or `otelcol.exporter.otlp`
- [ ] Document any extra steps (module cache directory, permissions, etc.) in the README
