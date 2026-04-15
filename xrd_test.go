package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzeXRD(t *testing.T) {
	tests := []struct {
		name                  string
		xrdContent            string
		wantAPIVersion        string
		wantHasXPrefix        bool
		wantHasClaimNames     bool
		wantHasScope          bool
		wantRequiresMigration bool
	}{
		{
			name: "v1 XRD with X-prefix and claims",
			xrdContent: `apiVersion: apiextensions.crossplane.io/v1
kind: CompositeResourceDefinition
metadata:
  name: xdatabases.example.com
spec:
  group: example.com
  names:
    kind: XDatabase
    plural: xdatabases
  claimNames:
    kind: Database
    plural: databases
  versions:
    - name: v1alpha1
      served: true
      referenceable: true
`,
			wantAPIVersion:        "apiextensions.crossplane.io/v1",
			wantHasXPrefix:        true,
			wantHasClaimNames:     true,
			wantHasScope:          false,
			wantRequiresMigration: true,
		},
		{
			name: "v2 XRD already migrated",
			xrdContent: `apiVersion: apiextensions.crossplane.io/v2
kind: CompositeResourceDefinition
metadata:
  name: databases.example.com
spec:
  group: example.com
  scope: Namespaced
  names:
    kind: Database
    plural: databases
  versions:
    - name: v1alpha1
      served: true
      referenceable: true
`,
			wantAPIVersion:        "apiextensions.crossplane.io/v2",
			wantHasXPrefix:        false,
			wantHasClaimNames:     false,
			wantHasScope:          true,
			wantRequiresMigration: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpDir := t.TempDir()
			xrdFile := filepath.Join(tmpDir, "definition.yaml")
			if err := os.WriteFile(xrdFile, []byte(tt.xrdContent), 0o644); err != nil {
				t.Fatalf("failed to write temp file: %v", err)
			}

			// Analyze
			analysis, err := AnalyzeXRD(xrdFile, nil)
			if err != nil {
				t.Fatalf("AnalyzeXRD() error = %v", err)
			}

			// Check results
			if analysis.APIVersion != tt.wantAPIVersion {
				t.Errorf("APIVersion = %v, want %v", analysis.APIVersion, tt.wantAPIVersion)
			}
			if analysis.HasXPrefix != tt.wantHasXPrefix {
				t.Errorf("HasXPrefix = %v, want %v", analysis.HasXPrefix, tt.wantHasXPrefix)
			}
			if analysis.HasClaimNames != tt.wantHasClaimNames {
				t.Errorf("HasClaimNames = %v, want %v", analysis.HasClaimNames, tt.wantHasClaimNames)
			}
			if analysis.HasScope != tt.wantHasScope {
				t.Errorf("HasScope = %v, want %v", analysis.HasScope, tt.wantHasScope)
			}
			if analysis.RequiresMigration != tt.wantRequiresMigration {
				t.Errorf("RequiresMigration = %v, want %v", analysis.RequiresMigration, tt.wantRequiresMigration)
			}
		})
	}
}

func TestScopeDetection(t *testing.T) {
	tests := []struct {
		name                   string
		clusterScopedResources []ClusterScopedResource
		wantScope              string
	}{
		{
			name: "has cluster-scoped resources",
			clusterScopedResources: []ClusterScopedResource{
				{Kind: "Namespace", Line: 10},
			},
			wantScope: "Cluster",
		},
		{
			name:                   "no cluster-scoped resources",
			clusterScopedResources: []ClusterScopedResource{},
			wantScope:              "Namespaced",
		},
		{
			name: "mixed resources with cluster-scoped",
			clusterScopedResources: []ClusterScopedResource{
				{Kind: "ClusterRole", Line: 5},
				{Kind: "ClusterRoleBinding", Line: 15},
			},
			wantScope: "Cluster",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compAnalysis := &CompositionAnalysis{
				ClusterScopedResources: tt.clusterScopedResources,
			}

			xrdContent := `apiVersion: apiextensions.crossplane.io/v1
kind: CompositeResourceDefinition
metadata:
  name: xdatabases.example.com
spec:
  group: example.com
  names:
    kind: XDatabase
    plural: xdatabases
  versions:
    - name: v1alpha1
      served: true
`

			tmpDir := t.TempDir()
			xrdFile := filepath.Join(tmpDir, "definition.yaml")
			if err := os.WriteFile(xrdFile, []byte(xrdContent), 0o644); err != nil {
				t.Fatalf("failed to write temp file: %v", err)
			}

			analysis, err := AnalyzeXRD(xrdFile, compAnalysis)
			if err != nil {
				t.Fatalf("AnalyzeXRD() error = %v", err)
			}

			if analysis.RequiredScope != tt.wantScope {
				t.Errorf("RequiredScope = %v, want %v", analysis.RequiredScope, tt.wantScope)
			}
		})
	}
}

func TestMigrateXRD(t *testing.T) {
	xrdContent := `apiVersion: apiextensions.crossplane.io/v1
kind: CompositeResourceDefinition
metadata:
  name: xdatabases.example.com
spec:
  group: example.com
  names:
    kind: XDatabase
    plural: xdatabases
  claimNames:
    kind: Database
    plural: databases
  versions:
    - name: v1alpha1
      served: true
      referenceable: true
`

	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "definition.yaml")
	targetFile := filepath.Join(tmpDir, "definition-v2.yaml")

	if err := os.WriteFile(sourceFile, []byte(xrdContent), 0o644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	analysis := &XRDAnalysis{
		APIVersion:    "apiextensions.crossplane.io/v1",
		Kind:          "XDatabase",
		MetadataName:  "xdatabases.example.com",
		HasXPrefix:    true,
		HasClaimNames: true,
		HasScope:      false,
		RequiredScope: "Namespaced",
	}

	if err := MigrateXRD(sourceFile, targetFile, analysis); err != nil {
		t.Fatalf("MigrateXRD() error = %v", err)
	}

	// Verify migrated file was created
	if _, err := os.Stat(targetFile); os.IsNotExist(err) {
		t.Errorf("migrated file was not created")
	}

	// Read and verify content
	content, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("failed to read migrated file: %v", err)
	}

	contentStr := string(content)

	// Check for v2 API version
	if !contains(contentStr, "apiextensions.crossplane.io/v2") {
		t.Errorf("migrated file does not contain v2 API version")
	}

	// Check that claimNames was removed
	if contains(contentStr, "claimNames") {
		t.Errorf("migrated file still contains claimNames")
	}

	// Check that scope was added
	if !contains(contentStr, "scope:") {
		t.Errorf("migrated file does not contain scope field")
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && (s[:len(substr)] == substr || contains(s[1:], substr)))
}
