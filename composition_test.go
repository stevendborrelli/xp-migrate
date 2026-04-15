package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyzeProviderAPIGroups(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantCount int
		wantOld   string
		wantNew   string
	}{
		{
			name: "AWS family provider",
			content: `apiVersion: ec2.aws.upbound.io/v1beta1
kind: VPC
spec:
  forProvider:
    region: us-west-2`,
			wantCount: 1,
			wantOld:   "aws.upbound.io",
			wantNew:   "aws.m.upbound.io",
		},
		{
			name: "Azure family provider",
			content: `apiVersion: network.azure.upbound.io/v1beta1
kind: VirtualNetwork`,
			wantCount: 1,
			wantOld:   "azure.upbound.io",
			wantNew:   "azure.m.upbound.io",
		},
		{
			name: "Kubernetes provider",
			content: `apiVersion: kubernetes.crossplane.io/v1alpha2
kind: Object`,
			wantCount: 1,
			wantOld:   "kubernetes.crossplane.io",
			wantNew:   "kubernetes.m.crossplane.io",
		},
		{
			name:      "no provider migrations needed",
			content:   `apiVersion: v1\nkind: ConfigMap`,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := strings.Split(tt.content, "\n")
			groups := analyzeProviderAPIGroups(lines, ProviderMappings)

			if len(groups) != tt.wantCount {
				t.Errorf("got %d groups, want %d", len(groups), tt.wantCount)
			}

			if tt.wantCount > 0 {
				if groups[0].Current != tt.wantOld {
					t.Errorf("Current = %v, want %v", groups[0].Current, tt.wantOld)
				}
				if groups[0].Required != tt.wantNew {
					t.Errorf("Required = %v, want %v", groups[0].Required, tt.wantNew)
				}
			}
		})
	}
}

func TestAnalyzeDeletionPolicy(t *testing.T) {
	tests := []struct {
		name            string
		content         string
		wantCount       int
		wantValue       string
		wantReplacement string
	}{
		{
			name: "deletionPolicy Delete",
			content: `spec:
  forProvider:
    region: us-west-2
  deletionPolicy: Delete`,
			wantCount:       1,
			wantValue:       "Delete",
			wantReplacement: `managementPolicies: ["*"]`,
		},
		{
			name: "deletionPolicy Orphan",
			content: `spec:
  forProvider:
    region: us-west-2
  deletionPolicy: Orphan`,
			wantCount:       1,
			wantValue:       "Orphan",
			wantReplacement: `managementPolicies: ["Observe", "Create", "Update", "LateInitialize"]`,
		},
		{
			name:      "no deletionPolicy",
			content:   `spec:\n  forProvider:\n    region: us-west-2`,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := strings.Split(tt.content, "\n")
			locations := analyzeDeletionPolicy(lines)

			if len(locations) != tt.wantCount {
				t.Errorf("got %d locations, want %d", len(locations), tt.wantCount)
			}

			if tt.wantCount > 0 {
				if locations[0].Value != tt.wantValue {
					t.Errorf("Value = %v, want %v", locations[0].Value, tt.wantValue)
				}
				if locations[0].Replacement != tt.wantReplacement {
					t.Errorf("Replacement = %v, want %v", locations[0].Replacement, tt.wantReplacement)
				}
			}
		})
	}
}

func TestAnalyzeClusterScopedResources(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantKinds []string
	}{
		{
			name: "has Namespace",
			content: `apiVersion: v1
kind: Namespace
metadata:
  name: test-namespace`,
			wantKinds: []string{"Namespace"},
		},
		{
			name: "has multiple cluster-scoped",
			content: `---
apiVersion: v1
kind: Namespace
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding`,
			wantKinds: []string{"Namespace", "ClusterRole", "ClusterRoleBinding"},
		},
		{
			name: "namespace-scoped only",
			content: `apiVersion: v1
kind: ConfigMap
---
apiVersion: v1
kind: Secret`,
			wantKinds: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := strings.Split(tt.content, "\n")
			resources := analyzeClusterScopedResources(lines)

			foundKinds := make(map[string]bool)
			for _, r := range resources {
				foundKinds[r.Kind] = true
			}

			if len(foundKinds) != len(tt.wantKinds) {
				t.Errorf("found %d unique kinds, want %d", len(foundKinds), len(tt.wantKinds))
			}

			for _, wantKind := range tt.wantKinds {
				if !foundKinds[wantKind] {
					t.Errorf("missing expected kind: %s", wantKind)
				}
			}
		})
	}
}

func TestAnalyzeComposition(t *testing.T) {
	compContent := `apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: xtenant.example.com
spec:
  compositeTypeRef:
    apiVersion: example.com/v1alpha1
    kind: XTenant
  mode: Pipeline
  pipeline:
    - step: create-resources
      functionRef:
        name: function-go-templating
      input:
        apiVersion: gotemplating.fn.crossplane.io/v1beta1
        kind: GoTemplate
        source: Inline
        inline:
          template: |
            apiVersion: kubernetes.crossplane.io/v1alpha2
            kind: Object
            spec:
              forProvider:
                manifest:
                  apiVersion: v1
                  kind: Namespace
              deletionPolicy: Delete
`

	tmpDir := t.TempDir()
	compFile := filepath.Join(tmpDir, "composition.yaml")
	if err := os.WriteFile(compFile, []byte(compContent), 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	analysis, err := AnalyzeComposition(compFile)
	if err != nil {
		t.Fatalf("AnalyzeComposition() error = %v", err)
	}

	if !analysis.CompositeTypeRef.HasXPrefix {
		t.Errorf("Expected HasXPrefix = true")
	}

	if len(analysis.ClusterScopedResources) == 0 {
		t.Errorf("Expected to find cluster-scoped Namespace resource")
	}

	if len(analysis.DeletionPolicyUsage) == 0 {
		t.Errorf("Expected to find deletionPolicy usage")
	}

	if len(analysis.KubernetesObjects) == 0 {
		t.Errorf("Expected to find Kubernetes Object")
	}

	if !analysis.RequiresMigration {
		t.Errorf("Expected RequiresMigration = true")
	}
}

func TestIsClusterScopedKind(t *testing.T) {
	tests := []struct {
		kind string
		want bool
	}{
		{"Namespace", true},
		{"ClusterRole", true},
		{"ClusterRoleBinding", true},
		{"ConfigMap", false},
		{"Secret", false},
		{"Deployment", false},
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			got := isClusterScopedKind(tt.kind)
			if got != tt.want {
				t.Errorf("isClusterScopedKind(%q) = %v, want %v", tt.kind, got, tt.want)
			}
		})
	}
}
