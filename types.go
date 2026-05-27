package main

import (
	"time"
)

// MigrationAnalysis represents the complete analysis of files.
type MigrationAnalysis struct {
	GeneratedAt  time.Time
	Path         string
	XRDs         []XRDAnalysis
	Compositions []CompositionAnalysis
	Functions    []FunctionAnalysis
	Examples     []ExampleAnalysis
	Summary      AnalysisSummary
}

// AnalysisSummary provides high-level statistics.
type AnalysisSummary struct {
	TotalXRDs               int
	TotalCompositions       int
	TotalFunctions          int
	TotalExamples           int
	XRDsRequiringMigration  int
	CompsRequiringMigration int
	FuncsRequiringUpdate    int
}

// XRDAnalysis contains analysis results for an XRD.
type XRDAnalysis struct {
	FilePath               string
	APIVersion             string
	Kind                   string
	MetadataName           string
	HasXPrefix             bool
	HasClaimNames          bool
	HasScope               bool
	CurrentScope           string
	RequiredScope          string
	ScopeReason            string
	ClusterScopedResources []string
	DefaultDeletePolicy    string
	RequiresMigration      bool
	Changes                []string
}

// CompositionAnalysis contains analysis results for a Composition.
type CompositionAnalysis struct {
	FilePath               string
	Name                   string
	CompositeTypeRef       CompositeTypeRef
	Mode                   string
	ProviderAPIGroups      []ProviderAPIGroup
	DeletionPolicyUsage    []DeletionPolicyLocation
	ProviderConfigRefs     []ProviderConfigRefLocation
	ClaimReferences        []ClaimReference
	ClusterScopedResources []ClusterScopedResource
	KubernetesObjects      []KubernetesObject
	EnvironmentConfigs     []EnvironmentConfigUsage
	FunctionReferences     []string
	RequiresMigration      bool
	Changes                []string
}

// CompositeTypeRef represents the composite type reference.
type CompositeTypeRef struct {
	APIVersion string
	Kind       string
	HasXPrefix bool
}

// ProviderAPIGroup represents a provider API group that needs migration.
type ProviderAPIGroup struct {
	Line     int
	Current  string
	Required string
	Service  string
	Count    int
}

// DeletionPolicyLocation tracks where deletionPolicy is used.
type DeletionPolicyLocation struct {
	Line        int
	Resource    string
	Value       string
	Replacement string
}

// ProviderConfigRefLocation tracks providerConfigRef usage.
type ProviderConfigRefLocation struct {
	Line     int
	Resource string
	HasKind  bool
}

// ClaimReference tracks claim name/namespace label references.
type ClaimReference struct {
	Line        int
	Type        string // "name" or "namespace"
	Current     string
	Replacement string
}

// ClusterScopedResource represents a cluster-scoped resource in composition.
type ClusterScopedResource struct {
	Line         int
	Kind         string
	ResourceName string
}

// KubernetesObject represents a Kubernetes Object resource.
type KubernetesObject struct {
	Line            int
	APIVersion      string
	RequiredVersion string
	WrappedKind     string
	IsClusterScoped bool
}

// EnvironmentConfigUsage tracks EnvironmentConfig references.
type EnvironmentConfigUsage struct {
	Line            string
	CurrentVersion  string
	RequiredVersion string
}

// FunctionAnalysis contains analysis for composition functions.
type FunctionAnalysis struct {
	FilePath        string
	Name            string
	CurrentVersion  string
	LatestVersion   string
	RequiresUpdate  bool
	BreakingChanges []string
}

// ExampleAnalysis contains analysis for example XRs/Claims.
type ExampleAnalysis struct {
	FilePath          string
	APIVersion        string
	Kind              string
	HasNamespace      bool
	RequiresMigration bool
	Changes           []string
}

// MigrationPlan represents the migration plan.
type MigrationPlan struct {
	Analysis    MigrationAnalysis
	XRDChanges  []XRDMigration
	CompChanges []CompositionMigration
	FuncChanges []FunctionMigration
	Summary     string
}

// XRDMigration represents changes to an XRD.
type XRDMigration struct {
	SourceFile string
	TargetFile string
	Changes    []Change
}

// CompositionMigration represents changes to a Composition.
type CompositionMigration struct {
	SourceFile string
	TargetFile string
	Changes    []Change
}

// FunctionMigration represents function version updates.
type FunctionMigration struct {
	SourceFile string
	TargetFile string
	Changes    []Change
}

// Change represents a specific change to be made.
type Change struct {
	Type        string // "replace", "add", "remove"
	Description string
	Path        string // YAML path
	OldValue    any
	NewValue    any
	Line        int
}

// ProviderMappings defines provider API group migrations.
var ProviderMappings = map[string]string{
	"aws.upbound.io":           "aws.m.upbound.io",
	"azure.upbound.io":         "azure.m.upbound.io",
	"gcp.upbound.io":           "gcp.m.upbound.io",
	"kubernetes.crossplane.io": "kubernetes.m.crossplane.io",
}

// FunctionVersions defines recommended function versions.
var FunctionVersions = map[string]string{
	"function-go-templating":       "v0.12.1",
	"function-auto-ready":          "v0.6.5",
	"function-extra-resources":     "v0.3.0",
	"function-patch-and-transform": "v0.10.6",
}

// ClusterScopedKinds lists Kubernetes cluster-scoped resource kinds.
var ClusterScopedKinds = []string{
	"APIService",
	"CertificateSigningRequest",
	"ClusterRole",
	"ClusterRoleBinding",
	"CSIDrivers",
	"CustomResourceDefinition",
	"IngressClasses",
	"MutatingWebhookConfiguration",
	"Namespace",
	"Node",
	"PersistentVolume",
	"PriorityClass",
	"PriorityLevelConfiguration",
	"RuntimeClass",
	"StorageClass",
	"ValidatingWebhookConfiguration",
	"VolumeAttachment",
}
