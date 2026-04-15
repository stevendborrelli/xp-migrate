package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// compositionDocument represents a Composition YAML structure.
type compositionDocument struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		CompositeTypeRef struct {
			APIVersion string `yaml:"apiVersion"`
			Kind       string `yaml:"kind"`
		} `yaml:"compositeTypeRef"`
		Mode string `yaml:"mode"`
	} `yaml:"spec"`
}

// AnalyzeComposition analyzes a Composition file for v2 migration requirements.
// Uses default provider API group mappings.
func AnalyzeComposition(filePath string) (*CompositionAnalysis, error) {
	return AnalyzeCompositionWithMappings(filePath, ProviderMappings)
}

// AnalyzeCompositionWithMappings analyzes a Composition file with custom provider API group mappings.
func AnalyzeCompositionWithMappings(filePath string, providerMappings map[string]string) (*CompositionAnalysis, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read composition file: %w", err)
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	var comp compositionDocument
	if err := yaml.Unmarshal(data, &comp); err != nil {
		return nil, fmt.Errorf("failed to parse composition YAML: %w", err)
	}

	// Check if this is a Composition
	if comp.Kind != "Composition" {
		return nil, fmt.Errorf("not a Composition")
	}

	analysis := &CompositionAnalysis{
		FilePath: filePath,
		Name:     comp.Metadata.Name,
		Mode:     comp.Spec.Mode,
		CompositeTypeRef: CompositeTypeRef{
			APIVersion: comp.Spec.CompositeTypeRef.APIVersion,
			Kind:       comp.Spec.CompositeTypeRef.Kind,
			HasXPrefix: strings.HasPrefix(comp.Spec.CompositeTypeRef.Kind, "X"),
		},
		Changes: []string{},
	}

	// Analyze provider API groups
	analysis.ProviderAPIGroups = analyzeProviderAPIGroups(lines, providerMappings)

	// Analyze deletionPolicy usage
	analysis.DeletionPolicyUsage = analyzeDeletionPolicy(lines)

	// Analyze providerConfigRef
	analysis.ProviderConfigRefs = analyzeProviderConfigRefs(content, lines)

	// Analyze claim references
	analysis.ClaimReferences = analyzeClaimReferences(content, lines)

	// Analyze cluster-scoped resources
	analysis.ClusterScopedResources = analyzeClusterScopedResources(lines)

	// Analyze Kubernetes Objects
	analysis.KubernetesObjects = analyzeKubernetesObjects(content, lines)

	// Analyze EnvironmentConfig usage
	analysis.EnvironmentConfigs = analyzeEnvironmentConfigs(content, lines)

	// Determine if migration is required
	analysis.RequiresMigration = needsCompositionMigration(analysis)

	// Build change list
	if analysis.RequiresMigration {
		buildCompositionChanges(analysis)
	}

	return analysis, nil
}

func analyzeProviderAPIGroups(lines []string, providerMappings map[string]string) []ProviderAPIGroup {
	var groups []ProviderAPIGroup
	seen := make(map[string]*ProviderAPIGroup)

	// Build patterns from provider mappings
	type patternInfo struct {
		old     string
		new     string
		pattern *regexp.Regexp
	}
	var patterns []patternInfo

	for oldDomain, newDomain := range providerMappings {
		// Create regex pattern to match the domain and optionally capture service prefix
		// Handle cases like "service.domain.io" and plain "domain.io"
		escapedDomain := regexp.QuoteMeta(oldDomain)

		// Try to match service prefix pattern (e.g., ec2.aws.upbound.io)
		servicePattern := regexp.MustCompile(`\b(\w+)\.` + escapedDomain)
		// Also match plain domain without service prefix
		plainPattern := regexp.MustCompile(`\b` + escapedDomain + `\b`)

		patterns = append(patterns, patternInfo{
			old:     oldDomain,
			new:     newDomain,
			pattern: servicePattern,
		})
		patterns = append(patterns, patternInfo{
			old:     oldDomain,
			new:     newDomain,
			pattern: plainPattern,
		})
	}

	for lineNum, line := range lines {
		for _, p := range patterns {
			if strings.Contains(line, p.old) {
				matches := p.pattern.FindStringSubmatch(line)
				service := ""
				if len(matches) > 1 {
					service = matches[1]
				}

				key := p.old + "-" + service
				if g, exists := seen[key]; exists {
					g.Count++
				} else {
					group := ProviderAPIGroup{
						Line:     lineNum + 1,
						Current:  p.old,
						Required: p.new,
						Service:  service,
						Count:    1,
					}
					groups = append(groups, group)
					seen[key] = &groups[len(groups)-1]
				}
				break // Avoid duplicate matches on same line
			}
		}
	}

	return groups
}

func analyzeDeletionPolicy(lines []string) []DeletionPolicyLocation {
	var locations []DeletionPolicyLocation

	deletionPolicyRegex := regexp.MustCompile(`^\s*deletionPolicy:\s*(\w+)`)

	for lineNum, line := range lines {
		if matches := deletionPolicyRegex.FindStringSubmatch(line); matches != nil {
			value := matches[1]
			replacement := ""
			switch value {
			case "Delete":
				replacement = `managementPolicies: ["*"]`
			case "Orphan":
				replacement = `managementPolicies: ["Observe", "Create", "Update", "LateInitialize"]`
			}

			locations = append(locations, DeletionPolicyLocation{
				Line:        lineNum + 1,
				Value:       value,
				Replacement: replacement,
			})
		}
	}

	return locations
}

func analyzeProviderConfigRefs(content string, lines []string) []ProviderConfigRefLocation {
	var locations []ProviderConfigRefLocation

	providerConfigRefRegex := regexp.MustCompile(`^\s*providerConfigRef:`)
	kindRegex := regexp.MustCompile(`^\s*kind:`)

	for lineNum, line := range lines {
		if providerConfigRefRegex.MatchString(line) {
			// Check if next few lines have 'kind:' field
			hasKind := false
			for i := lineNum + 1; i < len(lines) && i < lineNum+5; i++ {
				nextLine := lines[i]
				if kindRegex.MatchString(nextLine) {
					hasKind = true
					break
				}
				// Stop if we hit another top-level key
				if len(nextLine) > 0 && nextLine[0] != ' ' && nextLine[0] != '\t' {
					break
				}
			}

			locations = append(locations, ProviderConfigRefLocation{
				Line:    lineNum + 1,
				HasKind: hasKind,
			})
		}
	}

	return locations
}

func analyzeClaimReferences(content string, lines []string) []ClaimReference {
	var refs []ClaimReference

	namespaceRegex := regexp.MustCompile(`crossplane\.io/claim-namespace`)
	nameRegex := regexp.MustCompile(`crossplane\.io/claim-name`)

	for lineNum, line := range lines {
		if namespaceRegex.MatchString(line) {
			refs = append(refs, ClaimReference{
				Line:        lineNum + 1,
				Type:        "namespace",
				Current:     `get $xr.metadata.labels "crossplane.io/claim-namespace"`,
				Replacement: "$xr.metadata.namespace",
			})
		}
		if nameRegex.MatchString(line) {
			refs = append(refs, ClaimReference{
				Line:        lineNum + 1,
				Type:        "name",
				Current:     `get $xr.metadata.labels "crossplane.io/claim-name"`,
				Replacement: "$xr.metadata.name",
			})
		}
	}

	return refs
}

func analyzeClusterScopedResources(lines []string) []ClusterScopedResource {
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

func analyzeKubernetesObjects(content string, lines []string) []KubernetesObject {
	var objects []KubernetesObject

	apiVersionRegex := regexp.MustCompile(`apiVersion:\s*(kubernetes\.(crossplane\.io|m\.crossplane\.io)/(v1(alpha|beta)\d+))`)

	for lineNum, line := range lines {
		if matches := apiVersionRegex.FindStringSubmatch(line); matches != nil {
			currentVersion := matches[1]
			requiredVersion := "kubernetes.m.crossplane.io/v1alpha1"

			// Look ahead for wrapped kind
			wrappedKind := ""
			isClusterScoped := false
			for i := lineNum + 1; i < len(lines) && i < lineNum+20; i++ {
				if strings.Contains(lines[i], "manifest:") {
					for j := i + 1; j < len(lines) && j < i+10; j++ {
						if matches := regexp.MustCompile(`kind:\s*(\w+)`).FindStringSubmatch(lines[j]); matches != nil {
							wrappedKind = matches[1]
							isClusterScoped = isClusterScopedKind(wrappedKind)
							break
						}
					}
					break
				}
			}

			objects = append(objects, KubernetesObject{
				Line:            lineNum + 1,
				APIVersion:      currentVersion,
				RequiredVersion: requiredVersion,
				WrappedKind:     wrappedKind,
				IsClusterScoped: isClusterScoped,
			})
		}
	}

	return objects
}

func analyzeEnvironmentConfigs(content string, lines []string) []EnvironmentConfigUsage {
	var configs []EnvironmentConfigUsage

	envConfigRegex := regexp.MustCompile(`apiextensions\.crossplane\.io/(v1alpha1).*EnvironmentConfig`)

	for lineNum, line := range lines {
		if matches := envConfigRegex.FindStringSubmatch(line); matches != nil {
			configs = append(configs, EnvironmentConfigUsage{
				Line:            fmt.Sprintf("%d", lineNum+1),
				CurrentVersion:  matches[1],
				RequiredVersion: "v1beta1",
			})
		}
	}

	return configs
}

func needsCompositionMigration(a *CompositionAnalysis) bool {
	return a.CompositeTypeRef.HasXPrefix ||
		len(a.ProviderAPIGroups) > 0 ||
		len(a.DeletionPolicyUsage) > 0 ||
		hasProviderConfigRefsWithoutKind(a.ProviderConfigRefs) ||
		len(a.ClaimReferences) > 0 ||
		len(a.EnvironmentConfigs) > 0 ||
		hasOldKubernetesAPI(a.KubernetesObjects)
}

func hasProviderConfigRefsWithoutKind(refs []ProviderConfigRefLocation) bool {
	for _, ref := range refs {
		if !ref.HasKind {
			return true
		}
	}
	return false
}

func hasOldKubernetesAPI(objects []KubernetesObject) bool {
	for _, obj := range objects {
		if obj.APIVersion != obj.RequiredVersion {
			return true
		}
	}
	return false
}

func buildCompositionChanges(a *CompositionAnalysis) {
	if a.CompositeTypeRef.HasXPrefix {
		newKind := strings.TrimPrefix(a.CompositeTypeRef.Kind, "X")
		a.Changes = append(a.Changes, fmt.Sprintf(
			"Update compositeTypeRef.kind: '%s' → '%s'", a.CompositeTypeRef.Kind, newKind))

		newName := strings.ToLower(newKind) + "s"
		a.Changes = append(a.Changes, fmt.Sprintf(
			"Update metadata.name to match new kind (remove X-prefix): e.g., → '%s'", newName))
	}

	if len(a.ProviderAPIGroups) > 0 {
		a.Changes = append(a.Changes, fmt.Sprintf(
			"**CRITICAL**: Update %d provider API group reference(s):", len(a.ProviderAPIGroups)))
		for _, g := range a.ProviderAPIGroups {
			a.Changes = append(a.Changes, fmt.Sprintf(
				"  - Line %d: '%s' → '%s' (service: %s, %d occurrence(s))",
				g.Line, g.Current, g.Required, g.Service, g.Count))
		}
	}

	if len(a.DeletionPolicyUsage) > 0 {
		a.Changes = append(a.Changes, fmt.Sprintf(
			"**CRITICAL**: Convert %d deletionPolicy field(s) to managementPolicies:", len(a.DeletionPolicyUsage)))
		for _, dp := range a.DeletionPolicyUsage {
			a.Changes = append(a.Changes, fmt.Sprintf(
				"  - Line %d: 'deletionPolicy: %s' → '%s'", dp.Line, dp.Value, dp.Replacement))
		}
	}

	missingKind := []ProviderConfigRefLocation{}
	for _, ref := range a.ProviderConfigRefs {
		if !ref.HasKind {
			missingKind = append(missingKind, ref)
		}
	}
	if len(missingKind) > 0 {
		a.Changes = append(a.Changes, fmt.Sprintf(
			"**CRITICAL**: Add 'kind: ProviderConfig' to %d providerConfigRef block(s):", len(missingKind)))
		for _, ref := range missingKind {
			a.Changes = append(a.Changes, fmt.Sprintf("  - Line %d", ref.Line))
		}
	}

	if len(a.ClaimReferences) > 0 {
		a.Changes = append(a.Changes, fmt.Sprintf(
			"Update %d claim name/namespace label reference(s):", len(a.ClaimReferences)))
		for _, ref := range a.ClaimReferences {
			a.Changes = append(a.Changes, fmt.Sprintf(
				"  - Line %d: '%s' → '%s'", ref.Line, ref.Current, ref.Replacement))
		}
	}

	if len(a.ClusterScopedResources) > 0 {
		kinds := make(map[string]bool)
		for _, r := range a.ClusterScopedResources {
			kinds[r.Kind] = true
		}
		kindList := []string{}
		for k := range kinds {
			kindList = append(kindList, k)
		}
		a.Changes = append(a.Changes, fmt.Sprintf(
			"⚠️ SCOPE IMPACT: Composition creates cluster-scoped resources (%s). XRD must use 'scope: Cluster'",
			strings.Join(kindList, ", ")))
	}

	if len(a.KubernetesObjects) > 0 {
		oldAPI := []KubernetesObject{}
		for _, obj := range a.KubernetesObjects {
			if obj.APIVersion != obj.RequiredVersion {
				oldAPI = append(oldAPI, obj)
			}
		}
		if len(oldAPI) > 0 {
			a.Changes = append(a.Changes, fmt.Sprintf(
				"Update %d Kubernetes Object API version(s):", len(oldAPI)))
			for _, obj := range oldAPI {
				a.Changes = append(a.Changes, fmt.Sprintf(
					"  - Line %d: '%s' → '%s' (wraps: %s)",
					obj.Line, obj.APIVersion, obj.RequiredVersion, obj.WrappedKind))
			}
		}
	}

	if len(a.EnvironmentConfigs) > 0 {
		a.Changes = append(a.Changes, fmt.Sprintf(
			"Update %d EnvironmentConfig API version(s) to v1beta1", len(a.EnvironmentConfigs)))
	}
}

func isClusterScopedKind(kind string) bool {
	return slices.Contains(ClusterScopedKinds, kind)
}

// FindCompositionFiles finds all Composition files in a directory.
func FindCompositionFiles(dir string) ([]string, error) {
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

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		if strings.Contains(string(data), "kind: Composition") {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

// MigrateComposition applies v2 migrations to a composition file.
func MigrateComposition(sourcePath string, targetPath string, analysis *CompositionAnalysis) error {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return fmt.Errorf("failed to parse composition YAML: %w", err)
	}

	// Update compositeTypeRef kind if it has X prefix using YAML node manipulation
	if analysis.CompositeTypeRef.HasXPrefix {
		if err := updateCompositeTypeRefKind(&node, analysis.CompositeTypeRef.Kind); err != nil {
			return fmt.Errorf("failed to update compositeTypeRef kind: %w", err)
		}
	}

	// Marshal the updated composition back to YAML
	updatedData, err := yaml.Marshal(&node)
	if err != nil {
		return fmt.Errorf("failed to marshal composition: %w", err)
	}

	content := string(updatedData)

	// Apply remaining text-based replacements for complex transformations
	// that are harder to do with structured data
	for _, group := range analysis.ProviderAPIGroups {
		content = strings.ReplaceAll(content, group.Current, group.Required)
	}

	// Replace deletionPolicy with managementPolicies
	for _, dp := range analysis.DeletionPolicyUsage {
		oldPattern := fmt.Sprintf("deletionPolicy: %s", dp.Value)
		content = strings.ReplaceAll(content, oldPattern, dp.Replacement)
	}

	// Update claim references
	content = strings.ReplaceAll(content, `crossplane.io/claim-namespace`, `CLAIM_NAMESPACE_REPLACED`)
	content = strings.ReplaceAll(content, `crossplane.io/claim-name`, `CLAIM_NAME_REPLACED`)

	if err := os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	return nil
}

// updateCompositeTypeRefKind updates the compositeTypeRef.kind in the YAML node.
func updateCompositeTypeRefKind(node *yaml.Node, oldKind string) error {
	if node.Kind != yaml.DocumentNode || len(node.Content) == 0 {
		return fmt.Errorf("expected document node")
	}

	root := node.Content[0]
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("expected mapping node at root")
	}

	// Find spec node
	specNode := findCompositionYAMLNode(root, "spec")
	if specNode == nil {
		return fmt.Errorf("spec node not found")
	}

	// Find compositeTypeRef node
	compositeTypeRefNode := findCompositionYAMLNode(specNode, "compositeTypeRef")
	if compositeTypeRefNode == nil {
		return fmt.Errorf("compositeTypeRef node not found")
	}

	// Update kind
	newKind := strings.TrimPrefix(oldKind, "X")
	updateCompositionYAMLValue(compositeTypeRefNode, "kind", newKind)

	return nil
}

// Helper functions for YAML manipulation in compositions

func findCompositionYAMLNode(parent *yaml.Node, key string) *yaml.Node {
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

func updateCompositionYAMLValue(parent *yaml.Node, key string, value string) {
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
