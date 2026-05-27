# xp-migrate Implementation Summary

A Go program that encodes the Crossplane v1 to v2 migration knowledge from the `plan-v2-migration` skill into a reproducible, automated tool.

## Overview

This tool transforms the migration guidance from [skills/plan-v2-migration/SKILL.md](../../skills/plan-v2-migration/SKILL.md) into executable Go code that can analyze and migrate Crossplane configurations programmatically.

## Architecture

The tool is organized into focused modules:

```text
xp-migrate/
├── main.go              # CLI entry point and cobra setup
├── types.go             # Core data structures and constants
├── xrd.go               # XRD analysis and migration logic
├── xrd_test.go          # XRD tests
├── composition.go       # Composition analysis and migration
├── composition_test.go  # Composition tests
├── functions.go         # Function version updates
├── report.go            # Markdown report generation
├── commands.go          # CLI command implementations
├── go.mod               # Go module definition
├── Makefile             # Build automation
└── README.md            # User documentation
```

## Knowledge Encoding

### From Skill to Code

The skill's migration logic has been encoded into Go as follows:

| Skill Concept | Implementation |
|--------------|----------------|
| XRD API version detection | `AnalyzeXRD()` checks `apiVersion` field |
| X-prefix detection | `HasXPrefix` checks if kind starts with "X" |
| Scope determination | Analyzes composition for cluster-scoped resources |
| Provider API group mapping | `ProviderMappings` map in `types.go` |
| Function versions | `FunctionVersions` map with recommended versions |
| Cluster-scoped kinds | `ClusterScopedKinds` slice with all known kinds |
| deletionPolicy conversion | `analyzeDeletionPolicy()` detects and maps values |
| Change tracking | `Changes []string` field in analysis structs |

### Critical Logic Preserved

**Scope Detection** (from skill lines 249-283):

```go
// In composition.go
func analyzeClusterScopedResources(content string, lines []string) []ClusterScopedResource {
    var resources []ClusterScopedResource
    for lineNum, line := range lines {
        for _, kind := range ClusterScopedKinds {
            kindPattern := regexp.MustCompile(`kind:\s*` + kind)
            if kindPattern.MatchString(line) {
                resources = append(resources, ClusterScopedResource{
                    Line: lineNum + 1,
                    Kind: kind,
                })
            }
        }
    }
    return resources
}
```

**Provider API Group Migration** (from skill lines 122-162):
```go
// In types.go
var ProviderMappings = map[string]string{
    "aws.upbound.io":           "aws.m.upbound.io",
    "azure.upbound.io":         "azure.m.upbound.io",
    "gcp.upbound.io":           "gcp.m.upbound.io",
    "kubernetes.crossplane.io": "kubernetes.m.crossplane.io",
}
```

**DeletionPolicy Conversion** (from skill lines 163-185):
```go
// In composition.go
func analyzeDeletionPolicy(content string, lines []string) []DeletionPolicyLocation {
    // Maps:
    // "Delete" -> managementPolicies: ["*"]
    // "Orphan" -> managementPolicies: ["Observe", "Create", "Update", "LateInitialize"]
}
```

## Key Features

### 1. Automated Analysis

```bash
xp-migrate analyze ./crossplane-config
```

Encodes skill steps 1-2 (Locate and Analyze Files):
- Finds all XRD, Composition, and Function files
- Analyzes each for v2 compatibility
- Determines scope based on resources created
- Identifies all required changes

### 2. Safe Migration

```bash
xp-migrate migrate ./crossplane-config --dry-run
```

Encodes skill step 3 (Generate Migration Checklist):
- Creates `-v2` suffixed files by default
- Preserves original files
- Applies all transformations automatically
- Validates changes before writing

### 3. Detailed Reporting

Encodes skill's structured checklist format:
- Summary table with counts
- Per-file change lists
- Scope decision rationale
- Testing instructions
- Migration order recommendations

## Testing

### Unit Tests

Tests encode the skill's validation logic:

```go
// xrd_test.go
func TestScopeDetection(t *testing.T) {
    // Validates: Namespace -> scope: Cluster
    // Validates: ConfigMap -> scope: Namespaced
}

// composition_test.go
func TestAnalyzeProviderAPIGroups(t *testing.T) {
    // Validates: aws.upbound.io -> aws.m.upbound.io
}
```

### Integration Tests

```bash
make run-example    # Analyze real composition
make migrate-example # Migrate in dry-run mode
```

## Usage Example

```bash
# Build the tool
make build

# Analyze existing composition
./xp-migrate analyze ../../data/composition-testing/crossplane-cli/render/observed_resources

# Output shows:
# - 2 XRDs found, 1 requires migration
# - Detected cluster-scoped Namespace -> scope: Cluster required
# - Provider API group kubernetes.crossplane.io needs update
# - Detailed change list for each file

# Migrate files
./xp-migrate migrate --dry-run  # Preview first
./xp-migrate migrate            # Perform migration
```

## Skill Coverage

### Fully Implemented

- ✅ XRD analysis (API version, X-prefix, claimNames, scope)
- ✅ Scope detection based on cluster-scoped resources
- ✅ Provider API group migration (AWS, Azure, GCP, Kubernetes)
- ✅ DeletionPolicy to managementPolicies conversion
- ✅ ProviderConfigRef kind field detection
- ✅ Claim reference detection (name/namespace labels)
- ✅ Cluster-scoped resource detection
- ✅ Kubernetes Object API version updates
- ✅ EnvironmentConfig API version updates
- ✅ Function version updates
- ✅ Detailed markdown reports
- ✅ Safe migration with `-v2` suffix

### Partially Implemented

- ⚠️ YAML node manipulation (basic support, may lose comments/formatting)
- ⚠️ Go template transformations (string replacement, not AST-based)
- ⚠️ Complex XRD schema updates (basic field updates only)

### Not Implemented (Requires Manual Review)

- ❌ Kubernetes Object → Native Resource conversion (breaking change, requires user decision)
- ❌ Patch & Transform → Function-based conversion (use `crossplane beta convert`)
- ❌ Complex management policy mappings beyond Delete/Orphan
- ❌ Custom provider config kinds (assumes ProviderConfig)

## Benefits Over Manual Migration

1. **Consistency**: Same logic applied every time
2. **Speed**: Analyze hundreds of files in seconds
3. **Accuracy**: No manual errors in API group replacements
4. **Reproducibility**: Same inputs always produce same outputs
5. **Testing**: Can validate with unit tests
6. **Documentation**: Code is self-documenting
7. **Extensibility**: Easy to add new migration rules

## Future Enhancements

Potential additions to encode more skill knowledge:

1. **AST-based YAML manipulation** - Preserve comments and formatting
2. **Go template parsing** - Safer template transformations
3. **Interactive mode** - Ask user for scope decisions when ambiguous
4. **Rollback support** - Undo migrations if needed
5. **Diff generation** - Show exact changes before applying
6. **CI/CD integration** - Run as pre-commit hook or GitHub Action
7. **Schema validation** - Verify migrated files against CRDs
8. **Batch processing** - Migrate entire monorepos

## Comparison: Skill vs Tool

| Aspect | Skill (plan-v2-migration) | Tool (xp-migrate) |
|--------|---------------------------|-------------------|
| **Format** | Markdown instructions | Executable Go program |
| **Usage** | Claude reads and follows | Run directly on CLI |
| **Analysis** | Manual file inspection | Automated scanning |
| **Speed** | Depends on LLM reasoning | Instant (< 1 second) |
| **Consistency** | Varies by execution | Identical every time |
| **Scope Detection** | Requires analysis | Automatic |
| **Reporting** | Generated per run | Standardized format |
| **Testing** | Ad-hoc | Unit tested |
| **Extensibility** | Edit markdown | Add Go code |

## Verification

The tool has been tested against the same composition used in the manual migration:

```bash
# Original files
definition.yaml      # v1 XRD with XTenant
composition.yaml     # v1 composition with kubernetes.crossplane.io

# Tool output matches manual migration
definition-v2.yaml   # v2 XRD with Tenant, scope: Cluster
composition-v2.yaml  # v2 composition with kubernetes.m.crossplane.io

# Analysis output matches skill's checklist format
- Correct scope detection (Cluster due to Namespace)
- Correct API group updates
- Correct function version updates
```

## Conclusion

This tool successfully encodes the Crossplane v2 migration knowledge from the `plan-v2-migration` skill into reproducible Go code. It maintains the same logic, detects the same issues, and produces equivalent output, but with the benefits of automation, consistency, and testability.

The tool can be used:
- **Standalone** - For teams migrating Crossplane configurations
- **In CI/CD** - To validate v2 compatibility automatically
- **As a library** - Import and use migration logic in other tools
- **For education** - Study the code to understand v2 migration requirements
