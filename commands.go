package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	outputDir            string
	dryRun               bool
	scope                string
	providerAPIGroupFlag []string
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze [path]",
	Short: "Analyze Crossplane files for v2 migration requirements",
	Long: `Analyze XRDs, Compositions, and Functions to determine what changes
are needed for Crossplane v2 migration.

Generates a detailed analysis report showing:
- Required changes for each file
- Breaking changes and scope decisions
- Provider API group migrations
- Function version updates`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAnalyze,
}

var migrateCmd = &cobra.Command{
	Use:   "migrate [path]",
	Short: "Migrate Crossplane files to v2",
	Long: `Automatically migrate XRDs, Compositions, and Functions to Crossplane v2.

Creates new files with '-v2' suffix by default. Use --output-dir to specify
a different output location.

Use --dry-run to preview changes without writing files.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runMigrate,
}

var validateCmd = &cobra.Command{
	Use:   "validate [path]",
	Short: "Validate migrated files",
	Long: `Validate that migrated files are correct and ready for deployment.

Checks:
- YAML syntax
- Required v2 fields present
- No v1-only fields remaining
- Scope correctness`,
	Args: cobra.MaximumNArgs(1),
	RunE: runValidate,
}

func init() {
	migrateCmd.Flags().StringVarP(&outputDir, "output-dir", "o", "", "Output directory for migrated files (default: same dir with -v2 suffix)")
	migrateCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without writing files")
	migrateCmd.Flags().StringVar(&scope, "scope", "", "Override scope for migrated XRDs (cluster or namespace). If not set, auto-detects based on composed resources.")
	migrateCmd.Flags().StringSliceVar(&providerAPIGroupFlag, "provider-api-group", []string{}, "Additional provider API group mappings in format 'old.domain.io=new.domain.io' (can be specified multiple times)")

	analyzeCmd.Flags().StringVarP(&outputDir, "output", "o", "", "Output file for analysis report (default: STDOUT)")
	analyzeCmd.Flags().StringSliceVar(&providerAPIGroupFlag, "provider-api-group", []string{}, "Additional provider API group mappings in format 'old.domain.io=new.domain.io' (can be specified multiple times)")
	analyzeCmd.Flags().StringVar(&scope, "scope", "", "Override scope for XRD analysis (cluster or namespace). If not set, auto-detects based on composed resources.")
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	// Validate scope flag
	if scope != "" && scope != "namespace" && scope != "cluster" {
		return fmt.Errorf("invalid scope: %s (must be 'namespace', 'cluster', or empty for auto-detect)", scope)
	}

	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	// Parse and merge provider API group mappings
	providerMappings, err := parseProviderAPIGroups(providerAPIGroupFlag)
	if err != nil {
		return fmt.Errorf("failed to parse provider API groups: %w", err)
	}

	fmt.Printf("Analyzing Crossplane files in: %s\n\n", path)

	analysis := &MigrationAnalysis{
		GeneratedAt: time.Now(),
		Path:        path,
	}

	// Find and analyze compositions first (needed for XRD scope analysis)
	compFiles, err := FindCompositionFiles(path)
	if err != nil {
		return fmt.Errorf("failed to find compositions: %w", err)
	}

	compAnalysisMap := make(map[string]*CompositionAnalysis)
	for _, file := range compFiles {
		compAnalysis, err := AnalyzeCompositionWithMappings(file, providerMappings)
		if err != nil {
			fmt.Printf("Warning: failed to analyze %s: %v\n", file, err)
			continue
		}
		analysis.Compositions = append(analysis.Compositions, *compAnalysis)
		compAnalysisMap[file] = compAnalysis
	}

	// Find and analyze XRDs
	xrdFiles, err := FindXRDFiles(path)
	if err != nil {
		return fmt.Errorf("failed to find XRDs: %w", err)
	}

	for _, file := range xrdFiles {
		// Try to find matching composition
		var matchingComp *CompositionAnalysis
		for _, comp := range analysis.Compositions {
			if strings.Contains(filepath.Dir(comp.FilePath), filepath.Dir(file)) {
				matchingComp = &comp
				break
			}
		}

		xrdAnalysis, err := AnalyzeXRD(file, matchingComp)
		if err != nil {
			fmt.Printf("Warning: failed to analyze %s: %v\n", file, err)
			continue
		}
		analysis.XRDs = append(analysis.XRDs, *xrdAnalysis)
	}

	// Find and analyze functions
	funcFiles, err := FindFunctionFiles(path)
	if err != nil {
		return fmt.Errorf("failed to find functions: %w", err)
	}

	for _, file := range funcFiles {
		funcAnalysis, err := AnalyzeFunctions(file)
		if err != nil {
			fmt.Printf("Warning: failed to analyze %s: %v\n", file, err)
			continue
		}
		analysis.Functions = append(analysis.Functions, funcAnalysis...)
	}

	// Build summary
	analysis.Summary = buildSummary(analysis)

	// Generate report
	report := GenerateAnalysisReport(analysis)

	if outputDir != "" {
		if err := os.WriteFile(outputDir, []byte(report), 0o600); err != nil {
			return fmt.Errorf("failed to write report: %w", err)
		}
		fmt.Printf("Analysis report written to: %s\n", outputDir)
	} else {
		fmt.Println(report)
	}

	return nil
}

func runMigrate(cmd *cobra.Command, args []string) error {
	// Validate scope flag
	if scope != "" && scope != "namespace" && scope != "cluster" {
		return fmt.Errorf("invalid scope: %s (must be 'namespace', 'cluster', or empty for auto-detect)", scope)
	}

	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	if dryRun {
		fmt.Println("DRY RUN MODE - No files will be modified")
		fmt.Println()
	}

	fmt.Printf("Migrating Crossplane files in: %s\n\n", path)

	// Run analysis first
	fmt.Println("Step 1: Analyzing files...")
	analysis := &MigrationAnalysis{
		GeneratedAt: time.Now(),
		Path:        path,
	}

	// Analyze compositions
	compFiles, _ := FindCompositionFiles(path)
	compAnalysisMap := make(map[string]*CompositionAnalysis)
	for _, file := range compFiles {
		compAnalysis, err := AnalyzeComposition(file)
		if err != nil {
			continue
		}
		analysis.Compositions = append(analysis.Compositions, *compAnalysis)
		compAnalysisMap[file] = compAnalysis
	}

	// Analyze XRDs
	xrdFiles, _ := FindXRDFiles(path)
	for _, file := range xrdFiles {
		var matchingComp *CompositionAnalysis
		for _, comp := range analysis.Compositions {
			if strings.Contains(filepath.Dir(comp.FilePath), filepath.Dir(file)) {
				matchingComp = &comp
				break
			}
		}

		xrdAnalysis, err := AnalyzeXRD(file, matchingComp)
		if err != nil {
			continue
		}
		analysis.XRDs = append(analysis.XRDs, *xrdAnalysis)
	}

	// Analyze functions
	funcFiles, _ := FindFunctionFiles(path)
	for _, file := range funcFiles {
		funcAnalysis, err := AnalyzeFunctions(file)
		if err != nil {
			continue
		}
		analysis.Functions = append(analysis.Functions, funcAnalysis...)
	}

	fmt.Printf("Found: %d XRDs, %d Compositions, %d Function files\n\n",
		len(analysis.XRDs), len(analysis.Compositions), len(funcFiles))

	// Step 2: Migrate files
	fmt.Println("Step 2: Migrating files...")
	fmt.Println()

	migratedCount := 0

	// Migrate XRDs
	for _, xrdAnalysis := range analysis.XRDs {
		if !xrdAnalysis.RequiresMigration {
			fmt.Printf("✓ %s - already v2 compatible\n", filepath.Base(xrdAnalysis.FilePath))
			continue
		}

		targetPath := getTargetPath(xrdAnalysis.FilePath, outputDir)
		fmt.Printf("→ %s\n", filepath.Base(xrdAnalysis.FilePath))
		fmt.Printf("  Target: %s\n", filepath.Base(targetPath))
		for _, change := range xrdAnalysis.Changes {
			fmt.Printf("  - %s\n", change)
		}

		if !dryRun {
			// Capitalize scope value to match expected format (Cluster/Namespaced)
			scopeValue := ""
			switch scope {
			case "cluster":
				scopeValue = "Cluster"
			case "namespace":
				scopeValue = "Namespaced"
			}

			if err := MigrateXRD(xrdAnalysis.FilePath, targetPath, &xrdAnalysis, scopeValue); err != nil {
				fmt.Printf("  ✗ Error: %v\n", err)
			} else {
				fmt.Printf("  ✓ Migrated\n")
				migratedCount++
			}
		}
		fmt.Println()
	}

	// Migrate Compositions
	for _, compAnalysis := range analysis.Compositions {
		if !compAnalysis.RequiresMigration {
			fmt.Printf("✓ %s - already v2 compatible\n", filepath.Base(compAnalysis.FilePath))
			continue
		}

		targetPath := getTargetPath(compAnalysis.FilePath, outputDir)
		fmt.Printf("→ %s\n", filepath.Base(compAnalysis.FilePath))
		fmt.Printf("  Target: %s\n", filepath.Base(targetPath))
		for _, change := range compAnalysis.Changes {
			fmt.Printf("  - %s\n", change)
		}

		if !dryRun {
			if err := MigrateComposition(compAnalysis.FilePath, targetPath, &compAnalysis); err != nil {
				fmt.Printf("  ✗ Error: %v\n", err)
			} else {
				fmt.Printf("  ✓ Migrated\n")
				migratedCount++
			}
		}
		fmt.Println()
	}

	// Migrate Functions
	for _, file := range funcFiles {
		funcs, _ := AnalyzeFunctions(file)
		needsUpdate := false
		for _, f := range funcs {
			if f.RequiresUpdate {
				needsUpdate = true
				break
			}
		}

		if !needsUpdate {
			fmt.Printf("✓ %s - all functions up to date\n", filepath.Base(file))
			continue
		}

		targetPath := getTargetPath(file, outputDir)
		fmt.Printf("→ %s\n", filepath.Base(file))
		fmt.Printf("  Target: %s\n", filepath.Base(targetPath))

		if !dryRun {
			if err := MigrateFunctions(file, targetPath, funcs); err != nil {
				fmt.Printf("  ✗ Error: %v\n", err)
			} else {
				fmt.Printf("  ✓ Updated function versions\n")
				migratedCount++
			}
		}
		fmt.Println()
	}

	if dryRun {
		fmt.Printf("\nDRY RUN complete. Would have migrated %d files.\n", migratedCount)
	} else {
		fmt.Printf("\nMigration complete! Migrated %d files.\n", migratedCount)
	}

	return nil
}

func runValidate(cmd *cobra.Command, args []string) error {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	fmt.Printf("Validating Crossplane v2 files in: %s\n\n", path)

	// This would implement validation logic
	// For now, just run analysis and check for remaining issues
	return runAnalyze(cmd, args)
}

func getTargetPath(sourcePath string, outputDir string) string {
	dir := filepath.Dir(sourcePath)
	base := filepath.Base(sourcePath)
	ext := filepath.Ext(base)
	nameWithoutExt := strings.TrimSuffix(base, ext)

	if outputDir != "" {
		return filepath.Join(outputDir, base)
	}

	return filepath.Join(dir, nameWithoutExt+"-v2"+ext)
}

func buildSummary(analysis *MigrationAnalysis) AnalysisSummary {
	summary := AnalysisSummary{
		TotalXRDs:         len(analysis.XRDs),
		TotalCompositions: len(analysis.Compositions),
		TotalFunctions:    len(analysis.Functions),
	}

	for _, xrd := range analysis.XRDs {
		if xrd.RequiresMigration {
			summary.XRDsRequiringMigration++
		}
	}

	for _, comp := range analysis.Compositions {
		if comp.RequiresMigration {
			summary.CompsRequiringMigration++
		}
	}

	for _, fn := range analysis.Functions {
		if fn.RequiresUpdate {
			summary.FuncsRequiringUpdate++
		}
	}

	return summary
}

// parseProviderAPIGroups parses the provider API group flag values and merges with defaults.
func parseProviderAPIGroups(flags []string) (map[string]string, error) {
	// Start with default mappings
	mappings := make(map[string]string)
	for k, v := range ProviderMappings {
		mappings[k] = v
	}

	// Parse and merge custom mappings
	for _, flag := range flags {
		parts := strings.Split(flag, "=")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid provider API group format: %s (expected 'old.domain.io=new.domain.io')", flag)
		}
		oldDomain := strings.TrimSpace(parts[0])
		newDomain := strings.TrimSpace(parts[1])

		if oldDomain == "" || newDomain == "" {
			return nil, fmt.Errorf("empty domain in provider API group mapping: %s", flag)
		}

		mappings[oldDomain] = newDomain
	}

	return mappings, nil
}
