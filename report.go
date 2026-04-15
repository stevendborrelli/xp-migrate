package main

import (
	"fmt"
	"strings"
)

// GenerateAnalysisReport generates a detailed markdown report.
func GenerateAnalysisReport(analysis *MigrationAnalysis) string {
	var sb strings.Builder

	sb.WriteString("# Crossplane v2 Migration Analysis\n\n")
	fmt.Fprintf(&sb, "**Generated**: %s\n", analysis.GeneratedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&sb, "**Analyzed path**: %s\n\n", analysis.Path)

	// Summary table
	sb.WriteString("## Summary\n\n")
	sb.WriteString("| Category | Total | Require Migration |\n")
	sb.WriteString("|----------|-------|-------------------|\n")
	fmt.Fprintf(&sb, "| XRDs | %d | %d |\n",
		analysis.Summary.TotalXRDs, analysis.Summary.XRDsRequiringMigration)
	fmt.Fprintf(&sb, "| Compositions | %d | %d |\n",
		analysis.Summary.TotalCompositions, analysis.Summary.CompsRequiringMigration)
	fmt.Fprintf(&sb, "| Functions | %d | %d |\n",
		analysis.Summary.TotalFunctions, analysis.Summary.FuncsRequiringUpdate)
	sb.WriteString("\n")

	// XRD Analysis
	if len(analysis.XRDs) > 0 {
		sb.WriteString("## XRD Analysis\n\n")
		for i, xrd := range analysis.XRDs {
			fmt.Fprintf(&sb, "### %d. %s\n\n", i+1, xrd.FilePath)
			fmt.Fprintf(&sb, "- **API Version**: %s\n", xrd.APIVersion)
			fmt.Fprintf(&sb, "- **Kind**: %s\n", xrd.Kind)
			fmt.Fprintf(&sb, "- **Metadata Name**: %s\n", xrd.MetadataName)
			fmt.Fprintf(&sb, "- **Has X-Prefix**: %t\n", xrd.HasXPrefix)
			fmt.Fprintf(&sb, "- **Has ClaimNames**: %t\n", xrd.HasClaimNames)
			fmt.Fprintf(&sb, "- **Current Scope**: %s\n", xrd.CurrentScope)
			fmt.Fprintf(&sb, "- **Required Scope**: %s\n", xrd.RequiredScope)

			if len(xrd.ClusterScopedResources) > 0 {
				fmt.Fprintf(&sb, "- **⚠️ Cluster-Scoped Resources**: %s\n",
					strings.Join(xrd.ClusterScopedResources, ", "))
				fmt.Fprintf(&sb, "- **Scope Reason**: %s\n", xrd.ScopeReason)
			}

			fmt.Fprintf(&sb, "- **Requires Migration**: %t\n\n", xrd.RequiresMigration)

			if len(xrd.Changes) > 0 {
				sb.WriteString("**Required Changes:**\n")
				for _, change := range xrd.Changes {
					fmt.Fprintf(&sb, "- %s\n", change)
				}
				sb.WriteString("\n")
			}
		}
	}

	// Composition Analysis
	if len(analysis.Compositions) > 0 {
		sb.WriteString("## Composition Analysis\n\n")
		for i, comp := range analysis.Compositions {
			fmt.Fprintf(&sb, "### %d. %s\n\n", i+1, comp.FilePath)
			fmt.Fprintf(&sb, "- **Name**: %s\n", comp.Name)
			fmt.Fprintf(&sb, "- **Mode**: %s\n", comp.Mode)
			fmt.Fprintf(&sb, "- **Composite Kind**: %s (apiVersion: %s)\n",
				comp.CompositeTypeRef.Kind, comp.CompositeTypeRef.APIVersion)
			fmt.Fprintf(&sb, "- **Has X-Prefix**: %t\n", comp.CompositeTypeRef.HasXPrefix)
			fmt.Fprintf(&sb, "- **Requires Migration**: %t\n\n", comp.RequiresMigration)

			// Provider API Groups
			if len(comp.ProviderAPIGroups) > 0 {
				sb.WriteString("**Provider API Group Migrations:**\n")
				for _, g := range comp.ProviderAPIGroups {
					fmt.Fprintf(&sb, "- Line %d: `%s` → `%s` (service: %s, count: %d)\n",
						g.Line, g.Current, g.Required, g.Service, g.Count)
				}
				sb.WriteString("\n")
			}

			// DeletionPolicy
			if len(comp.DeletionPolicyUsage) > 0 {
				sb.WriteString("**⚠️ DeletionPolicy Conversions:**\n")
				for _, dp := range comp.DeletionPolicyUsage {
					fmt.Fprintf(&sb, "- Line %d: `deletionPolicy: %s` → `%s`\n",
						dp.Line, dp.Value, dp.Replacement)
				}
				sb.WriteString("\n")
			}

			// ProviderConfigRef
			missingKind := 0
			for _, ref := range comp.ProviderConfigRefs {
				if !ref.HasKind {
					missingKind++
				}
			}
			if missingKind > 0 {
				fmt.Fprintf(&sb, "**⚠️ ProviderConfigRef Missing Kind**: %d occurrences\n", missingKind)
				sb.WriteString("Add `kind: ProviderConfig` to all providerConfigRef blocks\n\n")
			}

			// Claim References
			if len(comp.ClaimReferences) > 0 {
				sb.WriteString("**Claim Reference Updates:**\n")
				for _, ref := range comp.ClaimReferences {
					fmt.Fprintf(&sb, "- Line %d (%s): `%s` → `%s`\n",
						ref.Line, ref.Type, ref.Current, ref.Replacement)
				}
				sb.WriteString("\n")
			}

			// Cluster-Scoped Resources
			if len(comp.ClusterScopedResources) > 0 {
				sb.WriteString("**⚠️ Cluster-Scoped Resources Created:**\n")
				kindMap := make(map[string][]int)
				for _, r := range comp.ClusterScopedResources {
					kindMap[r.Kind] = append(kindMap[r.Kind], r.Line)
				}
				for kind, lines := range kindMap {
					fmt.Fprintf(&sb, "- %s (lines: %v)\n", kind, lines)
				}
				sb.WriteString("\n**Impact**: XRD must use `scope: Cluster`\n\n")
			}

			// Kubernetes Objects
			if len(comp.KubernetesObjects) > 0 {
				sb.WriteString("**Kubernetes Object Resources:**\n")
				for _, obj := range comp.KubernetesObjects {
					fmt.Fprintf(&sb, "- Line %d: %s (wraps: %s",
						obj.Line, obj.APIVersion, obj.WrappedKind)
					if obj.IsClusterScoped {
						sb.WriteString(", ⚠️ cluster-scoped")
					}
					sb.WriteString(")\n")
					if obj.APIVersion != obj.RequiredVersion {
						fmt.Fprintf(&sb, "  → Update to: %s\n", obj.RequiredVersion)
					}
				}
				sb.WriteString("\n")
			}

			// Environment Configs
			if len(comp.EnvironmentConfigs) > 0 {
				sb.WriteString("**EnvironmentConfig Updates:**\n")
				for _, ec := range comp.EnvironmentConfigs {
					fmt.Fprintf(&sb, "- Line %s: %s → %s\n",
						ec.Line, ec.CurrentVersion, ec.RequiredVersion)
				}
				sb.WriteString("\n")
			}

			// All Changes
			if len(comp.Changes) > 0 {
				sb.WriteString("**All Required Changes:**\n")
				for _, change := range comp.Changes {
					fmt.Fprintf(&sb, "- %s\n", change)
				}
				sb.WriteString("\n")
			}
		}
	}

	// Function Analysis
	if len(analysis.Functions) > 0 {
		sb.WriteString("## Function Version Updates\n\n")

		fileMap := make(map[string][]FunctionAnalysis)
		for _, fn := range analysis.Functions {
			fileMap[fn.FilePath] = append(fileMap[fn.FilePath], fn)
		}

		for file, funcs := range fileMap {
			fmt.Fprintf(&sb, "### %s\n\n", file)
			for _, fn := range funcs {
				fmt.Fprintf(&sb, "- **%s**: %s → %s",
					fn.Name, fn.CurrentVersion, fn.LatestVersion)
				if fn.RequiresUpdate {
					sb.WriteString(" ⚠️ Update recommended")
				} else {
					sb.WriteString(" ✓ Up to date")
				}
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
		}
	}

	// Migration Recommendations
	sb.WriteString("## Migration Recommendations\n\n")

	if analysis.Summary.XRDsRequiringMigration == 0 &&
		analysis.Summary.CompsRequiringMigration == 0 &&
		analysis.Summary.FuncsRequiringUpdate == 0 {
		sb.WriteString("✅ **All files are v2 compatible!** No migration needed.\n\n")
	} else {
		sb.WriteString("### Migration Order\n\n")
		sb.WriteString("1. **Update Function versions** (safe, backward compatible)\n")
		sb.WriteString("2. **Migrate XRDs** (create new v2 XRDs alongside v1)\n")
		sb.WriteString("3. **Migrate Compositions** (update to reference v2 XRDs)\n")
		sb.WriteString("4. **Test locally** using `crossplane render`\n")
		sb.WriteString("5. **Deploy to test environment**\n")
		sb.WriteString("6. **Migrate existing claims** one at a time\n\n")

		sb.WriteString("### Critical Considerations\n\n")

		// Check for scope issues
		hasClusterScoped := false
		for _, comp := range analysis.Compositions {
			if len(comp.ClusterScopedResources) > 0 {
				hasClusterScoped = true
				break
			}
		}
		if hasClusterScoped {
			sb.WriteString("- ⚠️ **Scope Decision Required**: Some compositions create cluster-scoped resources. ")
			sb.WriteString("The corresponding XRDs MUST use `scope: Cluster`. ")
			sb.WriteString("Using `scope: Namespaced` will cause rendering failures.\n\n")
		}

		// Check for provider migrations
		hasProviderMigration := false
		for _, comp := range analysis.Compositions {
			if len(comp.ProviderAPIGroups) > 0 {
				hasProviderMigration = true
				break
			}
		}
		if hasProviderMigration {
			sb.WriteString("- ⚠️ **Provider Migration Required**: You'll need to install managed providers ")
			sb.WriteString("(e.g., provider-awsm, provider-azurem) alongside existing family providers ")
			sb.WriteString("during the migration period.\n\n")
		}

		// Check for deletionPolicy
		hasDeletionPolicy := false
		for _, comp := range analysis.Compositions {
			if len(comp.DeletionPolicyUsage) > 0 {
				hasDeletionPolicy = true
				break
			}
		}
		if hasDeletionPolicy {
			sb.WriteString("- ⚠️ **Breaking Change**: `deletionPolicy` field has been removed in v2. ")
			sb.WriteString("All resources must use `managementPolicies` instead. ")
			sb.WriteString("Schema validation will fail if `deletionPolicy` remains.\n\n")
		}
	}

	// Testing Instructions
	sb.WriteString("## Testing Your Migration\n\n")
	sb.WriteString("Before deploying to a cluster, test locally:\n\n")
	sb.WriteString("```bash\n")
	sb.WriteString("# Basic render test\n")
	sb.WriteString("crossplane render \\\n")
	sb.WriteString("  --xrd=definition-v2.yaml \\\n")
	sb.WriteString("  --include-full-xr \\\n")
	sb.WriteString("  xr.yaml composition-v2.yaml functions-v2.yaml\n\n")
	sb.WriteString("# With schema validation\n")
	sb.WriteString("crossplane render \\\n")
	sb.WriteString("  --xrd=definition-v2.yaml \\\n")
	sb.WriteString("  --include-full-xr \\\n")
	sb.WriteString("  xr.yaml composition-v2.yaml functions-v2.yaml | \\\n")
	sb.WriteString("  crossplane beta validate schemas -\n")
	sb.WriteString("```\n\n")

	return sb.String()
}

// GenerateMigrationSummary creates a brief summary for console output.
func GenerateMigrationSummary(analysis *MigrationAnalysis) string {
	var sb strings.Builder

	sb.WriteString("Migration Analysis Summary:")
	fmt.Fprintf(&sb, "  XRDs: %d total, %d require migration\n",
		analysis.Summary.TotalXRDs, analysis.Summary.XRDsRequiringMigration)
	fmt.Fprintf(&sb, "  Compositions: %d total, %d require migration\n",
		analysis.Summary.TotalCompositions, analysis.Summary.CompsRequiringMigration)
	fmt.Fprintf(&sb, "  Functions: %d total, %d require updates\n",
		analysis.Summary.TotalFunctions, analysis.Summary.FuncsRequiringUpdate)

	return sb.String()
}
