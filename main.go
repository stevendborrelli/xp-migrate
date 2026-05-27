// migrates crosplane v1 compositions to v2
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "0.1.0"

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "xp-migrate",
	Short: "Crossplane v1 to v2 migration tool",
	Long: `A tool to analyze and migrate Crossplane v1 configurations to v2.

This tool automates the migration of:
- CompositeResourceDefinitions (XRDs)
- Compositions
- Provider API groups (AWS, Azure, GCP family to managed)
- Function versions
- Claims and example XRs

Based on Upbound's Crossplane v2 migration best practices.`,
	Version: version,
}

func init() {
	rootCmd.AddCommand(analyzeCmd)
	rootCmd.AddCommand(migrateCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(initConfigCmd)
}
