package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// functionDocument represents a Function YAML structure for parsing.
// This mirrors the upstream Crossplane Function type from
// github.com/crossplane/crossplane/apis/pkg/v1
type functionDocument struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		Package string `yaml:"package"`
	} `yaml:"spec"`
}

// AnalyzeFunctions analyzes a functions.yaml file.
func AnalyzeFunctions(filePath string) ([]FunctionAnalysis, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read functions file: %w", err)
	}

	// Parse as multiple documents using typed structure
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	var functions []FunctionAnalysis

	for {
		var doc functionDocument
		if err := decoder.Decode(&doc); err != nil {
			break
		}

		// Skip non-Function kinds
		if doc.Kind != "Function" {
			continue
		}

		currentVersion := extractVersion(doc.Spec.Package)
		functionName := extractFunctionName(doc.Spec.Package)
		latestVersion := FunctionVersions[functionName]

		requiresUpdate := false
		if latestVersion != "" && currentVersion != latestVersion {
			requiresUpdate = true
		}

		functions = append(functions, FunctionAnalysis{
			FilePath:       filePath,
			Name:           doc.Metadata.Name,
			CurrentVersion: currentVersion,
			LatestVersion:  latestVersion,
			RequiresUpdate: requiresUpdate,
		})
	}

	return functions, nil
}

// MigrateFunctions updates function versions in a functions.yaml file.
func MigrateFunctions(sourcePath string, targetPath string, analysis []FunctionAnalysis) error {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	content := string(data)

	// Replace versions
	for _, fn := range analysis {
		if !fn.RequiresUpdate {
			continue
		}

		functionName := extractFunctionName(fn.Name)
		if fn.LatestVersion == "" {
			continue
		}

		// Replace version in package URL
		oldPattern := fmt.Sprintf("%s:%s", functionName, fn.CurrentVersion)
		newPattern := fmt.Sprintf("%s:%s", functionName, fn.LatestVersion)
		content = strings.ReplaceAll(content, oldPattern, newPattern)
	}

	if err := os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	return err
}

func extractVersion(pkg string) string {
	re := regexp.MustCompile(`:v(\d+\.\d+\.\d+)`)
	matches := re.FindStringSubmatch(pkg)
	if len(matches) > 1 {
		return "v" + matches[1]
	}
	return ""
}

func extractFunctionName(pkg string) string {
	// Extract function name from package URL or metadata name
	// e.g., "xpkg.upbound.io/crossplane-contrib/function-go-templating:v0.5.0"
	// or "crossplane-contrib-function-go-templating"

	parts := strings.Split(pkg, "/")
	if len(parts) > 0 {
		lastPart := parts[len(parts)-1]
		// Remove version
		lastPart = strings.Split(lastPart, ":")[0]
		return lastPart
	}

	// Try to extract from metadata name format
	if strings.Contains(pkg, "function-") {
		re := regexp.MustCompile(`(function-[\w-]+)`)
		matches := re.FindStringSubmatch(pkg)
		if len(matches) > 1 {
			return matches[1]
		}
	}

	return pkg
}

// FindFunctionFiles finds all function definition files.
func FindFunctionFiles(dir string) ([]string, error) {
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Look for functions.yaml or function.yaml files
		base := filepath.Base(path)
		if base == "functions.yaml" || base == "function.yaml" {
			files = append(files, path)
			return nil
		}

		// Also check any .yaml file that contains Function kind
		if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			if strings.Contains(string(data), "kind: Function") {
				files = append(files, path)
			}
		}

		return nil
	})

	return files, err
}
