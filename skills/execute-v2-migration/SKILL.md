---
name: execute-v2-migration
description: >-
  This skill should be used when the user asks to apply a v2 migration, execute v2 changes,
  implement the v2 migration, or convert Crossplane files to v2. Uses the xp-migrate CLI tool
  to automatically migrate XRDs, Compositions, and Functions to Crossplane v2. The tool creates
  new files with a '-v2' suffix by default, preserving the original files. Use this after
  running plan-v2-migration (or xp-migrate analyze) to implement the migration.
allowed-tools: Bash, Read, Write, Edit, Glob, Grep, AskUserQuestion
model: inherit
context: fork
agent: general-purpose
user-invocable: false
metadata:
  version: 0.1.0
  author: crossplane-contrib
  updated: 2026-05-27
  domains:
    - crossplane
    - migration
    - v2
    - platform-engineering
---

# Crossplane v2 Migration Executor (using xp-migrate)

Execute Crossplane v2 migrations using the `xp-migrate migrate` CLI tool.

**Safety**: Creates new files with `-v2` suffix by default, preserving original files.
**Tool**: Uses `xp-migrate migrate` to perform automated migration.
**Validation**: Use `xp-migrate validate` or `crossplane render` to verify changes.

---

## Prerequisites

The `xp-migrate` CLI tool must be installed and available in your PATH.

### Installing xp-migrate

If not already installed, build from source:

```bash
# In the xp-migrate directory
go build -o xp-migrate .

# Optionally move to PATH
sudo mv xp-migrate /usr/local/bin/
```

---

## Overview

This skill executes the migration in the following phases:

1. **Verify Prerequisites** - Check xp-migrate is available
2. **Dry Run (Optional)** - Preview changes without modifying files
3. **Execute Migration** - Run xp-migrate migrate to transform files
4. **Validate** - Test migrated files using crossplane render
5. **Report** - Summarize changes and next steps

---

## Phase 1: Verify Prerequisites

### Step 1.1: Check xp-migrate Installation

```bash
which xp-migrate || command -v xp-migrate
```

If not found, attempt to build:

```bash
if [ -f "main.go" ] && grep -q "xp-migrate" main.go 2>/dev/null; then
  echo "Building xp-migrate from source..."
  go build -o xp-migrate .
  export PATH="$PWD:$PATH"
fi
```

### Step 1.2: Confirm Path with User

Ask the user for the path to migrate:

```
Where are your Crossplane configuration files?
1. Current directory
2. Specific path — please provide it
```

---

## Phase 2: Preview Changes (Dry Run)

Before making any changes, run a dry-run to preview what will be modified:

```bash
xp-migrate migrate <path> --dry-run
```

### Dry Run Output

The dry-run shows:
- Which files will be migrated
- What changes will be applied to each file
- Target file paths (with `-v2` suffix)

Example output:
```
DRY RUN MODE - No files will be modified

Migrating Crossplane files in: ./crossplane

Step 1: Analyzing files...
Found: 2 XRDs, 2 Compositions, 1 Function files

Step 2: Migrating files...

→ xrd.yaml
  Target: xrd-v2.yaml
  - Update apiVersion to v2
  - Remove X-prefix from kind
  - Remove claimNames
  - Add scope: Namespaced

→ composition.yaml
  Target: composition-v2.yaml
  - Update compositeTypeRef kind
  - Migrate provider API groups (5 resources)
  - Convert deletionPolicy to managementPolicies

DRY RUN complete. Would have migrated 3 files.
```

### Ask User to Proceed

After showing the dry-run output:

```
The dry-run shows the changes that will be made.

Would you like to proceed with the migration?
1. Yes, execute the migration
2. No, I need to review further
3. Execute with custom output directory
```

---

## Phase 3: Execute Migration

### Basic Migration

Creates new files with `-v2` suffix in the same directory:

```bash
xp-migrate migrate <path>
```

### Migration with Custom Output Directory

Place all migrated files in a specific directory:

```bash
xp-migrate migrate <path> --output-dir ./v2-output
```

### Migration with Scope Override

Force a specific scope for all XRDs:

```bash
# Force all XRDs to use Namespaced scope
xp-migrate migrate <path> --scope namespace

# Force all XRDs to use Cluster scope
xp-migrate migrate <path> --scope cluster
```

### Migration with Custom Provider Mappings

Add custom provider API group mappings:

```bash
xp-migrate migrate <path> \
  --provider-api-group "custom.provider.io=custom.m.provider.io"
```

---

## Phase 4: What Gets Migrated

### XRD Transformations

For each XRD file:

1. **apiVersion**: `apiextensions.crossplane.io/v1` → `apiextensions.crossplane.io/v2`

2. **X-prefix removal**:
   - `kind: XDatabase` → `kind: Database`
   - `plural: xdatabases` → `plural: databases`
   - `metadata.name: xdatabases.group` → `metadata.name: databases.group`

3. **claimNames**: Entire block removed

4. **scope**: Added based on composed resources:
   - `scope: Cluster` if composition creates Namespace, ClusterRole, etc.
   - `scope: Namespaced` otherwise (default)

### Composition Transformations

For each Composition file:

1. **metadata.name**: X-prefix removed

2. **compositeTypeRef.kind**: X-prefix removed

3. **Provider API Groups**:
   - `*.aws.upbound.io` → `*.aws.m.upbound.io`
   - `*.azure.upbound.io` → `*.azure.m.upbound.io`
   - `*.gcp.upbound.io` → `*.gcp.m.upbound.io`
   - `kubernetes.crossplane.io` → `kubernetes.m.crossplane.io`

4. **deletionPolicy → managementPolicies**:
   - `deletionPolicy: Delete` → `managementPolicies: ["*"]`
   - `deletionPolicy: Orphan` → `managementPolicies: [Observe, Create, Update, LateInitialize]`

5. **providerConfigRef.kind**: Added `kind: ProviderConfig` if missing

### Function Transformations

For each function file:

- Updates function package versions to latest stable:
  - `function-go-templating` → v0.12.1
  - `function-auto-ready` → v0.6.5
  - `function-extra-resources` → v0.3.0
  - `function-patch-and-transform` → v0.10.6

---

## Phase 5: Validate Migrated Files

After migration, validate the changes:

### Using xp-migrate validate

```bash
xp-migrate validate <migrated-path>
```

### Using crossplane render

Test that compositions render correctly:

```bash
crossplane render \
  --xrd=xrd-v2.yaml \
  xr.yaml \
  composition-v2.yaml \
  functions.yaml \
  --include-full-xr
```

### Schema Validation (Optional)

For full schema validation against provider CRDs:

```bash
# Create schemas directory with provider packages
mkdir -p schemas
cat > schemas/providers.yaml <<EOF
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: upbound-provider-aws-ec2
spec:
  package: xpkg.upbound.io/upbound/provider-aws-ec2:v2.4.0
EOF

# Run render + validate pipeline
crossplane render \
  --xrd=xrd-v2.yaml \
  xr.yaml \
  composition-v2.yaml \
  functions.yaml \
  --include-full-xr | crossplane beta validate schemas -
```

---

## Phase 6: Migration Summary

After migration completes, provide a summary:

```markdown
## Migration Summary

### Files Created

| Original | Migrated |
|----------|----------|
| xrd.yaml | xrd-v2.yaml |
| composition.yaml | composition-v2.yaml |
| functions.yaml | functions-v2.yaml |

### Changes Applied

**XRDs:**
- Updated apiVersion v1 → v2
- Removed X-prefix from kinds
- Removed claimNames blocks
- Added scope field

**Compositions:**
- Updated compositeTypeRef kinds
- Migrated provider API groups to managed (.m.)
- Converted deletionPolicy to managementPolicies
- Added kind: ProviderConfig to providerConfigRefs

**Functions:**
- Updated to latest stable versions

### Next Steps

1. **Review migrated files** for correctness
2. **Test locally**:
   ```bash
   crossplane render --xrd=xrd-v2.yaml xr.yaml composition-v2.yaml functions-v2.yaml --include-full-xr
   ```
3. **Apply to cluster**:
   ```bash
   kubectl apply -f xrd-v2.yaml
   kubectl apply -f composition-v2.yaml
   kubectl apply -f functions-v2.yaml
   ```
4. **Verify deployment**:
   ```bash
   crossplane beta trace <resource-name> -n <namespace>
   ```

### Rollback (if needed)

Original files are preserved. To rollback, simply use the original files:
- `xrd.yaml` (original v1)
- `composition.yaml` (original v1)
- `functions.yaml` (original)
```

---

## Command Reference

### xp-migrate migrate

```
Usage:
  xp-migrate migrate [path] [flags]

Flags:
  -o, --output-dir string          Output directory for migrated files (default: same dir with -v2 suffix)
      --dry-run                    Preview changes without writing files
      --scope string               Override scope for XRDs (cluster or namespace)
      --provider-api-group strings Additional provider API group mappings (old=new format)

Examples:
  xp-migrate migrate .
  xp-migrate migrate ./crossplane --dry-run
  xp-migrate migrate . --output-dir ./v2
  xp-migrate migrate . --scope namespace
  xp-migrate migrate . --provider-api-group "custom.io=custom.m.io"
```

### xp-migrate validate

```
Usage:
  xp-migrate validate [path] [flags]

Description:
  Validates that migrated files are correct and ready for deployment.

  Checks:
  - YAML syntax
  - Required v2 fields present
  - No v1-only fields remaining
  - Scope correctness

Examples:
  xp-migrate validate ./v2-output
  xp-migrate validate .
```

---

## Error Handling

### Common Errors

| Error | Cause | Resolution |
|-------|-------|------------|
| `xp-migrate: command not found` | CLI not installed | Build from source: `go build -o xp-migrate .` |
| `failed to find XRDs` | No XRD files in path | Verify path contains Crossplane files |
| `failed to parse YAML` | Invalid YAML syntax | Fix syntax errors in source file |
| `scope is required` | Auto-detection failed | Use `--scope` flag to specify |

### Recovery

If migration produces unexpected results:

1. Original files are preserved (not modified)
2. Delete the `-v2` files and re-run with corrected flags
3. Use `--dry-run` to preview before committing

---

## Advanced Usage

### Migrating Specific Files

To migrate only certain files, organize them in a subdirectory:

```bash
mkdir -p to-migrate
cp specific-xrd.yaml specific-composition.yaml to-migrate/
xp-migrate migrate to-migrate/
```

### Custom Provider Mappings

For custom or third-party providers:

```bash
xp-migrate migrate . \
  --provider-api-group "mycompany.io=mycompany.m.io" \
  --provider-api-group "partner.io=partner.m.io"
```

### Force Cluster Scope

When you know all XRDs need cluster scope:

```bash
xp-migrate migrate . --scope cluster
```

### Output to Separate Directory

Keep v1 and v2 files separate:

```bash
xp-migrate migrate ./v1 --output-dir ./v2
```

---

## Integration with CI/CD

Example GitHub Actions workflow:

```yaml
name: Migrate to v2
on:
  workflow_dispatch:
    inputs:
      dry_run:
        description: 'Dry run only'
        required: true
        default: 'true'

jobs:
  migrate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.21'

      - name: Install xp-migrate
        run: |
          git clone https://github.com/crossplane-contrib/xp-migrate.git
          cd xp-migrate
          go build -o /usr/local/bin/xp-migrate .

      - name: Run migration
        run: |
          if [ "${{ inputs.dry_run }}" = "true" ]; then
            xp-migrate migrate . --dry-run
          else
            xp-migrate migrate . --output-dir ./v2-migrated
          fi

      - name: Upload migrated files
        if: inputs.dry_run != 'true'
        uses: actions/upload-artifact@v4
        with:
          name: v2-migrated
          path: ./v2-migrated/
```
