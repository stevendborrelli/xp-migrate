package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// xrdYAML represents the YAML structure of a CompositeResourceDefinition for parsing.
// This is a lightweight struct for YAML unmarshaling and analysis. We keep yaml.Node
// for versions to preserve raw YAML structure during transformations.
//
// Note: The official Crossplane v1 and v2 CompositeResourceDefinition types are available at:
// - github.com/upbound/xp-migrate/crossplane/apis/apiextensions/v1
// - github.com/upbound/xp-migrate/crossplane/apis/apiextensions/v2
// We use this simplified struct to avoid pulling in k8s.io dependencies and to maintain
// the ability to work directly with yaml.Node for transformation operations.
type xrdYAML struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		Group string `yaml:"group"`
		Names struct {
			Kind   string `yaml:"kind"`
			Plural string `yaml:"plural"`
		} `yaml:"names"`
		ClaimNames *struct {
			Kind   string `yaml:"kind"`
			Plural string `yaml:"plural"`
		} `yaml:"claimNames,omitempty"`
		Scope                        string      `yaml:"scope,omitempty"`
		DefaultCompositeDeletePolicy string      `yaml:"defaultCompositeDeletePolicy,omitempty"`
		Versions                     []yaml.Node `yaml:"versions"`
	} `yaml:"spec"`
}

// AnalyzeXRD analyzes an XRD file for v2 migration requirements.
func AnalyzeXRD(filePath string, compositionAnalysis *CompositionAnalysis) (*XRDAnalysis, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read XRD file: %w", err)
	}

	var xrd xrdYAML
	if err := yaml.Unmarshal(data, &xrd); err != nil {
		return nil, fmt.Errorf("failed to parse XRD YAML: %w", err)
	}

	if xrd.Kind != "CompositeResourceDefinition" {
		return nil, fmt.Errorf("not a CompositeResourceDefinition")
	}

	analysis := &XRDAnalysis{
		FilePath:            filePath,
		APIVersion:          xrd.APIVersion,
		Kind:                xrd.Spec.Names.Kind,
		MetadataName:        xrd.Metadata.Name,
		HasClaimNames:       xrd.Spec.ClaimNames != nil,
		HasScope:            xrd.Spec.Scope != "",
		CurrentScope:        xrd.Spec.Scope,
		DefaultDeletePolicy: xrd.Spec.DefaultCompositeDeletePolicy,
		Changes:             []string{},
	}

	// Check for X-prefix
	analysis.HasXPrefix = strings.HasPrefix(xrd.Spec.Names.Kind, "X")

	// Determine required scope based on composition analysis
	if compositionAnalysis != nil && len(compositionAnalysis.ClusterScopedResources) > 0 {
		analysis.RequiredScope = "Cluster"
		analysis.ScopeReason = fmt.Sprintf("Composition creates cluster-scoped resources: %s",
			strings.Join(getResourceKinds(compositionAnalysis.ClusterScopedResources), ", "))
		analysis.ClusterScopedResources = getResourceKinds(compositionAnalysis.ClusterScopedResources)
	} else {
		analysis.RequiredScope = "Namespaced"
		analysis.ScopeReason = "All resources in composition are namespace-scoped"
	}

	// Determine if migration is required
	analysis.RequiresMigration = needsXRDMigration(analysis)

	// Build change list
	if analysis.RequiresMigration {
		buildXRDChanges(analysis, &xrd)
	}

	return analysis, nil
}

// needsXRDMigration checks if XRD requires migration to v2.
func needsXRDMigration(a *XRDAnalysis) bool {
	return a.APIVersion != "apiextensions.crossplane.io/v2" ||
		a.HasXPrefix ||
		a.HasClaimNames ||
		!a.HasScope
}

// buildXRDChanges builds the list of changes needed for XRD migration.
func buildXRDChanges(a *XRDAnalysis, xrd *xrdYAML) {
	if a.APIVersion != "apiextensions.crossplane.io/v2" {
		a.Changes = append(a.Changes, fmt.Sprintf(
			"Update apiVersion from '%s' to 'apiextensions.crossplane.io/v2'",
			a.APIVersion))
	}

	if a.HasXPrefix {
		newKind := strings.TrimPrefix(a.Kind, "X")
		a.Changes = append(a.Changes, fmt.Sprintf(
			"Remove X-prefix from kind: '%s' → '%s'", a.Kind, newKind))

		// Also update metadata.name
		newName := strings.Replace(a.MetadataName, "x"+strings.ToLower(xrd.Spec.Names.Plural),
			strings.ToLower(xrd.Spec.Names.Plural), 1)
		a.Changes = append(a.Changes, fmt.Sprintf(
			"Update metadata.name: '%s' → '%s'", a.MetadataName, newName))
	}

	if a.HasClaimNames {
		a.Changes = append(a.Changes, "Remove spec.claimNames block (claims removed in v2)")
	}

	if !a.HasScope {
		a.Changes = append(a.Changes, fmt.Sprintf(
			"Add spec.scope: %s (%s)", a.RequiredScope, a.ScopeReason))
	} else if a.CurrentScope != a.RequiredScope {
		a.Changes = append(a.Changes, fmt.Sprintf(
			"WARNING: Change scope from '%s' to '%s' (%s)",
			a.CurrentScope, a.RequiredScope, a.ScopeReason))
	}

	if a.DefaultDeletePolicy != "" {
		a.Changes = append(a.Changes, fmt.Sprintf(
			"Note: defaultCompositeDeletePolicy '%s' is unchanged (field name same in v2)",
			a.DefaultDeletePolicy))
	}
}

// MigrateXRD migrates an XRD file to v2.
func MigrateXRD(sourcePath string, targetPath string, analysis *XRDAnalysis, scopeOverride ...string) error {
	// Apply scope override if provided
	if len(scopeOverride) > 0 && scopeOverride[0] != "" {
		analysis.RequiredScope = scopeOverride[0]
	}
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Apply transformations
	if err := transformXRDNode(&node, analysis); err != nil {
		return fmt.Errorf("failed to transform XRD: %w", err)
	}

	// Write migrated file
	output, err := yaml.Marshal(&node)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}

	if err := os.WriteFile(targetPath, output, 0o644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	return nil
}

// transformXRDNode applies v2 transformations to the YAML node.
func transformXRDNode(node *yaml.Node, analysis *XRDAnalysis) error {
	if node.Kind != yaml.DocumentNode {
		return fmt.Errorf("expected document node")
	}

	if len(node.Content) == 0 {
		return fmt.Errorf("empty document")
	}

	root := node.Content[0]
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("expected mapping node at root")
	}

	// Update apiVersion
	updateYAMLValue(root, "apiVersion", "apiextensions.crossplane.io/v2")

	// Find spec node
	specNode := findYAMLNode(root, "spec")
	if specNode == nil {
		return fmt.Errorf("spec node not found")
	}

	// Remove claimNames
	if analysis.HasClaimNames {
		removeYAMLNode(specNode, "claimNames")
	}

	// Add or update scope
	if !analysis.HasScope || analysis.CurrentScope != analysis.RequiredScope {
		insertYAMLValue(specNode, "scope", analysis.RequiredScope, afterKey("group"))
	}

	// Update names.kind and plural (remove X-prefix)
	if analysis.HasXPrefix {
		namesNode := findYAMLNode(specNode, "names")
		if namesNode != nil {
			newKind := strings.TrimPrefix(analysis.Kind, "X")
			updateYAMLValue(namesNode, "kind", newKind)

			// Also update plural to remove x-prefix
			// Get current plural value to determine the pattern
			for i := 0; i < len(namesNode.Content); i += 2 {
				if namesNode.Content[i].Value == "plural" {
					currentPlural := namesNode.Content[i+1].Value
					// Remove x-prefix from plural (e.g., "xapplications" -> "applications")
					if strings.HasPrefix(strings.ToLower(currentPlural), "x") {
						newPlural := currentPlural[1:]
						updateYAMLValue(namesNode, "plural", newPlural)
					}
					break
				}
			}
		}

		// Update metadata.name
		metadataNode := findYAMLNode(root, "metadata")
		if metadataNode != nil {
			oldPlural := strings.ToLower(analysis.Kind) + "s"
			newPlural := strings.ToLower(strings.TrimPrefix(analysis.Kind, "X")) + "s"
			newName := strings.Replace(analysis.MetadataName, oldPlural, newPlural, 1)
			updateYAMLValue(metadataNode, "name", newName)
		}
	}

	return nil
}

// Helper functions for YAML manipulation

func findYAMLNode(parent *yaml.Node, key string) *yaml.Node {
	if parent.Kind != yaml.MappingNode {
		return nil
	}

	for i := 0; i < len(parent.Content); i += 2 {
		if parent.Content[i].Value == key {
			return parent.Content[i+1]
		}
	}
	return nil
}

func updateYAMLValue(parent *yaml.Node, key string, value string) {
	if parent.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i < len(parent.Content); i += 2 {
		if parent.Content[i].Value == key {
			parent.Content[i+1].Value = value
			return
		}
	}
}

func removeYAMLNode(parent *yaml.Node, key string) {
	if parent.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i < len(parent.Content); i += 2 {
		if parent.Content[i].Value == key {
			// Remove both key and value nodes
			parent.Content = append(parent.Content[:i], parent.Content[i+2:]...)
			return
		}
	}
}

type insertPosition struct {
	afterKey string
}

func afterKey(key string) insertPosition {
	return insertPosition{afterKey: key}
}

func insertYAMLValue(parent *yaml.Node, key string, value string, pos insertPosition) {
	if parent.Kind != yaml.MappingNode {
		return
	}

	// Check if key already exists
	for i := 0; i < len(parent.Content); i += 2 {
		if parent.Content[i].Value == key {
			parent.Content[i+1].Value = value
			return
		}
	}

	// Create new nodes
	keyNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: key,
	}
	valueNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: value,
	}

	// Find insertion position
	insertIdx := len(parent.Content)
	if pos.afterKey != "" {
		for i := 0; i < len(parent.Content); i += 2 {
			if parent.Content[i].Value == pos.afterKey {
				insertIdx = i + 2
				break
			}
		}
	}

	// Insert at position
	parent.Content = append(parent.Content[:insertIdx],
		append([]*yaml.Node{keyNode, valueNode}, parent.Content[insertIdx:]...)...)
}

func getResourceKinds(resources []ClusterScopedResource) []string {
	kinds := make(map[string]bool)
	result := []string{}

	for _, r := range resources {
		if !kinds[r.Kind] {
			kinds[r.Kind] = true
			result = append(result, r.Kind)
		}
	}
	return result
}

// FindXRDFiles finds all XRD files in a directory.
func FindXRDFiles(dir string) ([]string, error) {
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}

		// Quick check if file contains CompositeResourceDefinition
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		if strings.Contains(string(data), "CompositeResourceDefinition") {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}
