---
name: plan-v2-migration
description: >-
  This skill should be used when the user asks to migrate to Crossplane v2, upgrade to v2, understand
  what changed in Crossplane v2, update XRDs for v2, understand v2 breaking changes — including removing
  the X-prefix from XRD kinds, removing claimNames, adding scope: Namespaced, or migrating from
  provider-aws to provider-awsm or provider-azure to provider-azurem. Also applies when the user is
  updating a Crossplane configuration from v1 to v2 or asks how to upgrade Crossplane. Uses the
  xp-migrate CLI tool to analyze existing XRDs, Compositions, and config files and generate a
  structured migration report.
allowed-tools: Bash, Read, Write, Glob, Grep, AskUserQuestion
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

# Crossplane v2 Migration Planner (using xp-migrate)

Analyze Crossplane v1 configuration files and generate a structured migration report for upgrading to v2 using the `xp-migrate` CLI tool.

**Scope**: ANALYSIS ONLY — generates a report. Does not modify files.
**Tool**: Uses `xp-migrate analyze` to perform automated analysis.

---

## Prerequisites

The `xp-migrate` CLI tool must be installed and available in your PATH.

### Installing xp-migrate

If not already installed, build from source:

```bash
# Clone the repository (if not already present)
git clone https://github.com/stevendborrelli/xp-migrate.git
cd xp-migrate

# Build the binary
go build -o xp-migrate .

# Move to PATH (optional)
sudo mv xp-migrate /usr/local/bin/
```

Or if in the xp-migrate directory:

```bash
go build -o xp-migrate .
```

---

## Terminology Disambiguation

These terms refer to distinct things — do not conflate them:

| Term                    | Meaning                                                       |
| ----------------------- | ------------------------------------------------------------- |
| **Crossplane v2**       | The overall new version with namespaced resources             |
| **XRD apiVersion v2**   | `apiextensions.crossplane.io/v2` (was `/v1`)                  |
| **Provider namespaced** | Providers using `.m.` in API groups: `awsm`, `azurem`, `gcpm` |
| **X-prefix removal**    | v1: `XNetwork` → v2: `Network` in XRD names/kinds             |
| **scope: Namespaced**   | New required field in v2 XRDs                                 |
| **claimNames removal**  | v2 XRDs have no claims — resources are natively namespaced    |

---

## Step 1: Verify xp-migrate Installation

First, check if xp-migrate is available:

```bash
which xp-migrate || command -v xp-migrate
```

If not found, check if we're in the xp-migrate repository and can build it:

```bash
if [ -f "main.go" ] && grep -q "xp-migrate" main.go 2>/dev/null; then
  echo "Building xp-migrate from source..."
  go build -o xp-migrate .
fi
```

If xp-migrate is not available and cannot be built, inform the user:

```
The xp-migrate CLI tool is required but not found.

Please install it by:
1. Building from source: go build -o xp-migrate . (in the xp-migrate directory)
2. Or adding the pre-built binary to your PATH

Would you like me to help you install it?
```

---

## Step 2: Locate Configuration Files

Ask the user for paths if not provided:

```
Where are your Crossplane configuration files?
1. Current directory (I'll search recursively)
2. Specific path — please provide it
```

Verify the path exists:

```bash
if [ ! -d "<path>" ]; then
  echo "ERROR: Directory not found: <path>"
fi
```

---

## Step 3: Run xp-migrate analyze

Execute the analysis using xp-migrate:

### Basic Analysis

```bash
xp-migrate analyze <path>
```

### Analysis with Custom Output File

```bash
xp-migrate analyze <path> -o CROSSPLANE_V2_MIGRATION.md
```

### Analysis with Custom Provider Mappings

If the user has custom providers that need migration:

```bash
xp-migrate analyze <path> \
  --provider-api-group "custom.provider.io=custom.m.provider.io"
```

### Analysis with Scope Override

If the user wants to force a specific scope:

```bash
xp-migrate analyze <path> --scope namespace
# or
xp-migrate analyze <path> --scope cluster
```

---

## Step 4: Review and Explain the Report

After running the analysis, review the generated report with the user.

### Key Sections to Explain

1. **Summary Table**: Shows counts of XRDs, Compositions, and Functions that need migration

2. **XRD Changes**: For each XRD file:
   - API version update (v1 → v2)
   - X-prefix removal from kind and plural names
   - claimNames removal
   - scope field addition (Cluster or Namespaced)

3. **Composition Changes**: For each Composition:
   - compositeTypeRef updates
   - Provider API group migrations (aws → awsm, azure → azurem, gcp → gcpm)
   - deletionPolicy → managementPolicies conversion
   - providerConfigRef.kind field addition

4. **Function Updates**: Recommended function version updates

5. **Scope Decision**: Explain the auto-detected scope:
   - If composition creates cluster-scoped resources (Namespace, ClusterRole, etc.) → `scope: Cluster`
   - Otherwise → `scope: Namespaced`

### Explaining Breaking Changes

Highlight critical breaking changes:

1. **Provider API Groups** (CRITICAL):
   - `*.aws.upbound.io` → `*.aws.m.upbound.io`
   - `*.azure.upbound.io` → `*.azure.m.upbound.io`
   - `*.gcp.upbound.io` → `*.gcp.m.upbound.io`
   - `kubernetes.crossplane.io` → `kubernetes.m.crossplane.io`

2. **deletionPolicy Removal** (CRITICAL):
   - `deletionPolicy: Delete` → `managementPolicies: ["*"]`
   - `deletionPolicy: Orphan` → `managementPolicies: [Observe, Create, Update, LateInitialize]`

3. **providerConfigRef.kind Required** (CRITICAL):
   - All providerConfigRef blocks must include `kind: ProviderConfig`

---

## Step 5: Save Report (if not already saved)

If the user wants to save the report to a file:

```bash
xp-migrate analyze <path> -o CROSSPLANE_V2_MIGRATION.md
echo "Report saved to CROSSPLANE_V2_MIGRATION.md"
```

---

## Step 6: Next Steps

After analysis is complete, inform the user of next steps:

````markdown
## Next Steps

Your migration analysis is complete. To proceed:

1. **Review the report** carefully, especially:
   - Scope decisions (Cluster vs Namespaced)
   - Provider API group migrations
   - Breaking changes

2. **Execute the migration** using:
   ```bash
   xp-migrate migrate <path>
   ```
````

Or for a dry-run preview:

```bash
xp-migrate migrate <path> --dry-run
```

3. **Validate the migrated files**:

   ```bash
   xp-migrate validate <path>
   ```

4. **Test locally** using crossplane render:
   ```bash
   crossplane render --xrd=xrd.yaml xr.yaml composition.yaml functions.yaml --include-full-xr
   ```

Would you like me to execute the migration now?

```

---

## Command Reference

### xp-migrate analyze

```

Usage:
xp-migrate analyze [path] [flags]

Flags:
-o, --output string Output file for analysis report (default: STDOUT)
--provider-api-group strings Additional provider API group mappings (old=new format)
--scope string Override scope (cluster or namespace, default: auto-detect)

Examples:
xp-migrate analyze .
xp-migrate analyze ./crossplane -o migration-plan.md
xp-migrate analyze . --scope cluster
xp-migrate analyze . --provider-api-group "custom.io=custom.m.io"

```

---

## What xp-migrate Analyzes

The tool automatically detects and analyzes:

### XRDs (CompositeResourceDefinitions)
- API version (v1 vs v2)
- X-prefix in kind/plural names
- claimNames presence
- Scope (missing, incorrect, or correct)
- defaultCompositeDeletePolicy

### Compositions
- compositeTypeRef kind references
- Provider API groups (AWS, Azure, GCP, Kubernetes)
- deletionPolicy usage
- providerConfigRef.kind presence
- Cluster-scoped resource detection
- Kubernetes Object resources
- EnvironmentConfig versions

### Functions
- Outdated function versions
- Recommended updates to latest stable versions:
  - function-go-templating → v0.12.1
  - function-auto-ready → v0.6.5
  - function-extra-resources → v0.3.0
  - function-patch-and-transform → v0.10.6

---

## Error Handling

If xp-migrate encounters errors:

1. **File not found**: Verify the path exists
2. **Parse errors**: Check YAML syntax in the file
3. **Permission errors**: Ensure read access to files

Report any errors to the user with suggested fixes.
```
