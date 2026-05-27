# xp-migrate - Crossplane v1 to v2 Migration Tool

A Go program that automates the analysis and migration of Crossplane v1 configurations to v2.

## Features

- **Automated Analysis**: Scans XRDs, Compositions, and Functions to identify required changes
- **Scope Detection**: Automatically determines whether XRDs should use `scope: Cluster` or `scope: Namespaced` based on resources created
- **Provider Migration**: Detects and updates provider API groups (AWS, Azure, GCP family → managed providers)
- **Breaking Change Detection**: Identifies `deletionPolicy`, missing `providerConfigRef.kind`, and other v2 incompatibilities
- **Safe Migration**: Creates new files with `-v2` suffix by default, preserving originals
- **Detailed Reports**: Generates comprehensive markdown reports with migration guidance

## Installation

```bash
go build -o xp-migrate
```

Or install directly:

```bash
go install github.com/stevendborrelli/xp-migrate@latest
```

## Usage

### Analyze Files

Analyze Crossplane files to see what needs to be migrated:

```bash
# Analyze current directory
xp-migrate analyze

# Analyze specific path
xp-migrate analyze ./my-crossplane-config

# Save report to file
xp-migrate analyze -o migration-report.md
```

### Migrate Files

Automatically migrate files to v2:

```bash
# Migrate files in current directory (auto-detect scope)
xp-migrate migrate

# Migrate specific path
xp-migrate migrate ./my-crossplane-config

# Dry run (preview changes without writing files)
xp-migrate migrate --dry-run

# Specify output directory
xp-migrate migrate -o ./v2-configs

# Override scope for all XRDs (cluster or namespace)
xp-migrate migrate --scope cluster
xp-migrate migrate --scope namespace
```

### Validate Files

Validate that migrated files are correct:

```bash
xp-migrate validate ./v2-configs
```

## What Gets Migrated

### XRDs (CompositeResourceDefinitions)

- ✅ Update `apiVersion` to `apiextensions.crossplane.io/v2`
- ✅ Remove X-prefix from kinds (`XDatabase` → `Database`)
- ✅ Remove X-prefix from plural names (`xdatabases` → `databases`)
- ✅ Update `metadata.name` (remove X-prefix from plural)
- ✅ Add `spec.scope: Namespaced` or `scope: Cluster` (auto-detected based on composition resources)
- ✅ Remove `spec.claimNames` block
- ✅ Preserve `defaultCompositeDeletePolicy` (field name unchanged in v2)

### Compositions

- ✅ Update `compositeTypeRef.kind` (remove X-prefix)
- ✅ Update provider API groups:
  - `*.aws.upbound.io` → `*.aws.m.upbound.io`
  - `*.azure.upbound.io` → `*.azure.m.upbound.io`
  - `*.gcp.upbound.io` → `*.gcp.m.upbound.io`
  - `kubernetes.crossplane.io` → `kubernetes.m.crossplane.io`
- ✅ Convert `deletionPolicy` to `managementPolicies`:
  - `deletionPolicy: Delete` → `managementPolicies: ["*"]`
  - `deletionPolicy: Orphan` → `managementPolicies: ["Observe", "Create", "Update", "LateInitialize"]`
- ✅ Add `kind: ProviderConfig` to all `providerConfigRef` blocks
- ✅ Update claim name/namespace label references to direct metadata access
- ✅ Update EnvironmentConfig API version to `v1beta1`

### Functions

- ✅ Update to latest stable versions:
  - `function-go-templating`: → v0.11.4
  - `function-auto-ready`: → v0.6.1
  - `function-extra-resources`: → v0.3.0
  - `function-patch-and-transform`: → v0.8.0

## Scope Detection

The tool automatically analyzes compositions to determine the correct scope for XRDs:

- **`scope: Namespaced`** (default) - Used if ALL resources are namespace-scoped:
  - ConfigMap, Secret, ServiceAccount, Deployment, Service, etc.

- **`scope: Cluster`** - Automatically detected if composition creates ANY cluster-scoped resources:
  - Namespace, ClusterRole, ClusterRoleBinding, PersistentVolume, StorageClass, etc.

**Override when needed**: Use `--scope cluster` or `--scope namespace` to manually override the auto-detected scope.

**Why this matters**: Using the wrong scope will cause rendering failures in v2.

## Example Output

### Analysis Report

```markdown
# Crossplane v2 Migration Analysis

**Generated**: 2026-03-06 14:30:00
**Analyzed path**: ./crossplane-config

## Summary

| Category | Total | Require Migration |
|----------|-------|-------------------|
| XRDs | 3 | 3 |
| Compositions | 3 | 3 |
| Functions | 1 | 1 |

## XRD Analysis

### 1. definition.yaml

- **API Version**: apiextensions.crossplane.io/v1
- **Kind**: XTenant
- **Required Scope**: Cluster
- **⚠️ Cluster-Scoped Resources**: Namespace
- **Scope Reason**: Composition creates cluster-scoped resources: Namespace

**Required Changes:**
- Update apiVersion from 'apiextensions.crossplane.io/v1' to 'apiextensions.crossplane.io/v2'
- Remove X-prefix from kind: 'XTenant' → 'Tenant'
- Add spec.scope: Cluster (Composition creates cluster-scoped resources: Namespace)
- Remove spec.claimNames block (claims removed in v2)
```

### Migration Output

```text
Migrating Crossplane files in: ./crossplane-config

Step 1: Analyzing files...
Found: 3 XRDs, 3 Compositions, 1 Function files

Step 2: Migrating files...

→ definition.yaml
  Target: definition-v2.yaml
  - Update apiVersion from 'apiextensions.crossplane.io/v1' to 'apiextensions.crossplane.io/v2'
  - Remove X-prefix from kind: 'XTenant' → 'Tenant'
  - Add spec.scope: Cluster
  - Remove spec.claimNames block
  ✓ Migrated

→ composition.yaml
  Target: composition-v2.yaml
  - Update compositeTypeRef.kind: 'XTenant' → 'Tenant'
  - **CRITICAL**: Update 3 provider API group reference(s)
  ✓ Migrated

Migration complete! Migrated 4 files.
```

## Migration Workflow

1. **Analyze** your configuration to understand required changes:

   ```bash
   xp-migrate analyze -o report.md
   ```

2. **Review** the analysis report to understand scope decisions and breaking changes

3. **Migrate** files with dry-run first:

   ```bash
   xp-migrate migrate --dry-run
   ```

4. **Migrate** for real:

   ```bash
   xp-migrate migrate
   ```

5. **Test locally** using Crossplane CLI:

   ```bash
   crossplane render \
     --xrd=definition-v2.yaml \
     --include-full-xr \
     xr.yaml composition-v2.yaml functions-v2.yaml
   ```

6. **Validate** with schema validation:

   ```bash
   crossplane render \
     --xrd=definition-v2.yaml \
     --include-full-xr \
     xr.yaml composition-v2.yaml functions-v2.yaml | \
     crossplane beta validate schemas -
   ```

7. **Deploy** to test environment

8. **Migrate** existing resources one at a time

## Recent Improvements

### Latest Updates

- ✅ Fixed critical bug where composition pipeline content was lost during migration
- ✅ Fixed XRD plural name not being updated when removing X-prefix
- ✅ Enhanced scope detection with auto-detection and manual override support
- ✅ All YAML content now preserved using yaml.Node for structural transformations

## Limitations

### Current Limitations

- **Composition metadata.name**: Not yet updated when removing X-prefix (identified in analysis)
- **Function names**: Not yet updated with `crossplane-contrib-` prefix (identified in analysis)
- **YAML Indentation**: Uses 4-space indentation (cosmetic difference)
- **Complex Templates**: Go template logic in compositions is migrated via string replacement - complex templates may need manual review
- **providerConfigRef.kind**: Added but may need adjustment for non-standard provider configs
- **Custom Resources**: Only handles standard Crossplane resources

### Manual Review Required

You should manually review:

- **Scope decisions** for XRDs with mixed resource types
- **Go template logic** that references XR kinds or claim metadata
- **ProviderConfigRef** blocks with custom kinds
- **Management policies** if you had custom deletionPolicy logic
- **Kubernetes Objects** - consider converting to native resources (breaking change, local cluster only)

## Architecture

The tool is organized into focused modules:

- `main.go` - CLI entry point
- `types.go` - Core data structures
- `xrd.go` - XRD analysis and migration
- `composition.go` - Composition analysis and migration
- `functions.go` - Function version updates
- `report.go` - Report generation
- `commands.go` - CLI command implementations

## Testing

Run tests:

```bash
go test -v ./...
```

Test with example data:

```bash
# Analyze example
xp-migrate analyze ../../data/composition-testing/crossplane-cli/render/observed_resources

# Migrate example
xp-migrate migrate ../../data/composition-testing/crossplane-cli/render/observed_resources --dry-run
```

## Contributing

Contributions welcome! This tool encodes the migration knowledge from Upbound's Crossplane v2 migration skill.

## References

- [Crossplane v2 Migration Guide](https://docs.crossplane.io/latest/concepts/v2-migration/)
- [Crossplane CLI Documentation](https://docs.crossplane.io/latest/cli/)
- [Upbound Crossplane Migration Skill](../../skills/plan-v2-migration/)

## License

Apache 2.0
