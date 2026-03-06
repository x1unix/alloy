# Alloy Forge — Progress

See [PLAN.md](./PLAN.md) for the full implementation plan.

## Step 1: Manifest parsing (`internal/forge/manifest`)

### T1.1 — Define core manifest structs

Create `internal/forge/manifest/manifest.go` with the following Go structs matching the YAML schema:

- [ ] `Manifest` — top-level: `Name`, `Stability`, `Community`, `Type`, `Source`
- [ ] `SourceConfig` — `Import`, `Version`, `PackageName`, `Dir`
- [ ] `OtelReceiverConfig` — `Factory`, `Config`
- [ ] `ConfigSchema` — `Type`, `Required`, `Properties`
- [ ] `PropertySchema` — `Type`, `Block`, `Required`, `Properties` (recursive, `map[string]PropertySchema`)

Use `yaml` struct tags throughout. `Properties` on `ConfigSchema` and `PropertySchema` should be `map[string]PropertySchema`.

### T1.2 — Define known enum values and constants

In the same package (or a `types.go` file), define:

- [ ] `ComponentType` string type with constants: `ComponentTypeAlloy`, `ComponentTypeOtelReceiver`, `ComponentTypeOtelExporter`
- [ ] `Stability` string type with constants: `StabilityExperimental`, `StabilityPublicPreview`, `StabilityGA`
- [ ] `PropertyType` string type with constants for `string`, `bool`, `int64`, `float64`, `object`

### T1.3 — Implement YAML unmarshaling

Add a `Parse(r io.Reader) (*Manifest, error)` function in `internal/forge/manifest/parse.go` that:

- [ ] Decodes YAML from the reader into `Manifest`
- [ ] Handles the `otelcol.receiver` YAML key (dot in key name requires special handling — either a custom `UnmarshalYAML` or a raw intermediate struct)

### T1.4 — Implement validation

Add `func (m *Manifest) Validate() error` that checks:

- [ ] `name` is non-empty and is a valid Alloy component name (dot-separated segments, each matching `[a-z][a-z0-9_]*`)
- [ ] `type` is one of the known `ComponentType` constants
- [ ] `stability` is one of the known `Stability` constants
- [ ] `source` has exactly one of `import` or `dir` set (not both, not neither)
- [ ] If `type` is `otelcol.receiver`, the `otelcol.receiver` block is present and `factory` is non-empty
- [ ] `config.type` is non-empty when a config block is present
- [ ] Required fields lists are non-empty slices of strings (no blank entries)

Return a descriptive `error` (not a panic) for each failure case.

### T1.5 — Unit tests

Create `internal/forge/manifest/manifest_test.go` covering:

- [ ] **Happy path**: parse `plugins/forge.source.awss3.yml` and assert all fields decode correctly, including nested `s3downloader` properties
- [ ] **Valid manifest**: a minimal valid `otelcol.receiver` manifest passes `Validate()`
- [ ] **Invalid `name`**: names like `""`, `"Forge.Source"`, `"forge..awss3"`, `"forge.source.awss3!"` are rejected
- [ ] **Unknown `type`**: `type: otelcol.processor` is rejected
- [ ] **Unknown `stability`**: `stability: beta` is rejected
- [ ] **Missing `source`**: both `import` and `dir` absent is rejected
- [ ] **Conflicting `source`**: both `import` and `dir` set is rejected
- [ ] **Missing factory**: `otelcol.receiver` block present but `factory` empty is rejected
- [ ] **Missing required block**: `type: otelcol.receiver` but no `otelcol.receiver` block is rejected

### Notes

- The dot in `otelcol.receiver` as a YAML key needs care. The standard `gopkg.in/yaml.v3` decoder will treat it as a plain string key, so a struct field tag `yaml:"otelcol.receiver"` works — just verify this with a quick test.
- `PropertySchema.Properties` being recursive (`map[string]PropertySchema`) means the type references itself; this is fine in Go but requires the map value to not be a pointer (or use `map[string]*PropertySchema` if you prefer).
- Keep `Parse` and `Validate` separate so callers can load manifests without validation (e.g. for tooling or testing partial manifests).
