package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the external configuration for xp-migrate.
type Config struct {
	// FunctionVersions maps function names to their recommended versions.
	FunctionVersions map[string]string `yaml:"functionVersions"`

	// ProviderMappings maps old provider API groups to new ones.
	ProviderMappings map[string]string `yaml:"providerMappings"`

	// ClusterScopedKinds lists additional cluster-scoped resource kinds.
	ClusterScopedKinds []string `yaml:"clusterScopedKinds"`
}

// DefaultConfigFileName is the default name for the config file.
const DefaultConfigFileName = "xp-migrate.yaml"

// DefaultFunctionVersionsFileName is the default name for function versions file.
const DefaultFunctionVersionsFileName = "function-versions.yaml"

// LoadConfig loads configuration from a file, merging with defaults.
// If configPath is empty, it looks for xp-migrate.yaml in the current directory
// and ~/.config/xp-migrate/xp-migrate.yaml.
func LoadConfig(configPath string) (*Config, error) {
	// Start with defaults
	cfg := &Config{
		FunctionVersions:   make(map[string]string),
		ProviderMappings:   make(map[string]string),
		ClusterScopedKinds: make([]string, 0),
	}

	// Copy default values
	for k, v := range DefaultFunctionVersions {
		cfg.FunctionVersions[k] = v
	}
	for k, v := range DefaultProviderMappings {
		cfg.ProviderMappings[k] = v
	}
	cfg.ClusterScopedKinds = append(cfg.ClusterScopedKinds, DefaultClusterScopedKinds...)

	// If explicit config path provided, use it
	if configPath != "" {
		if err := loadConfigFile(configPath, cfg); err != nil {
			return nil, fmt.Errorf("failed to load config from %s: %w", configPath, err)
		}
		return cfg, nil
	}

	// Try to load from standard locations (don't error if not found)
	configLocations := getConfigLocations()
	for _, loc := range configLocations {
		if _, err := os.Stat(loc); err == nil {
			if err := loadConfigFile(loc, cfg); err != nil {
				// Log warning but continue
				fmt.Fprintf(os.Stderr, "Warning: failed to load config from %s: %v\n", loc, err)
			}
		}
	}

	// Also try to load function-versions.yaml from standard locations
	for _, loc := range getFunctionVersionsLocations() {
		if _, err := os.Stat(loc); err == nil {
			if err := loadFunctionVersionsFile(loc, cfg); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to load function versions from %s: %v\n", loc, err)
			}
		}
	}

	return cfg, nil
}

// loadConfigFile loads a config file and merges it into cfg.
func loadConfigFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var fileCfg Config
	if err := yaml.Unmarshal(data, &fileCfg); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// Merge function versions (file overrides defaults)
	for k, v := range fileCfg.FunctionVersions {
		cfg.FunctionVersions[k] = v
	}

	// Merge provider mappings (file overrides defaults)
	for k, v := range fileCfg.ProviderMappings {
		cfg.ProviderMappings[k] = v
	}

	// Append cluster-scoped kinds (additive)
	if len(fileCfg.ClusterScopedKinds) > 0 {
		cfg.ClusterScopedKinds = appendUnique(cfg.ClusterScopedKinds, fileCfg.ClusterScopedKinds...)
	}

	return nil
}

// loadFunctionVersionsFile loads a function-versions.yaml file.
func loadFunctionVersionsFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var versions map[string]string
	if err := yaml.Unmarshal(data, &versions); err != nil {
		return fmt.Errorf("failed to parse function versions file: %w", err)
	}

	for k, v := range versions {
		cfg.FunctionVersions[k] = v
	}

	return nil
}

// getConfigLocations returns the list of config file locations to check.
func getConfigLocations() []string {
	locations := []string{
		DefaultConfigFileName, // Current directory
	}

	// Add ~/.config/xp-migrate/xp-migrate.yaml
	if home, err := os.UserHomeDir(); err == nil {
		locations = append(locations, filepath.Join(home, ".config", "xp-migrate", DefaultConfigFileName))
	}

	return locations
}

// getFunctionVersionsLocations returns locations to check for function-versions.yaml.
func getFunctionVersionsLocations() []string {
	locations := []string{
		DefaultFunctionVersionsFileName, // Current directory
	}

	// Add ~/.config/xp-migrate/function-versions.yaml
	if home, err := os.UserHomeDir(); err == nil {
		locations = append(locations, filepath.Join(home, ".config", "xp-migrate", DefaultFunctionVersionsFileName))
	}

	return locations
}

// appendUnique appends items to a slice, avoiding duplicates.
func appendUnique(slice []string, items ...string) []string {
	existing := make(map[string]bool)
	for _, s := range slice {
		existing[s] = true
	}
	for _, item := range items {
		if !existing[item] {
			slice = append(slice, item)
			existing[item] = true
		}
	}
	return slice
}

// GenerateDefaultConfig generates a default configuration file.
func GenerateDefaultConfig() string {
	cfg := Config{
		FunctionVersions:   DefaultFunctionVersions,
		ProviderMappings:   DefaultProviderMappings,
		ClusterScopedKinds: DefaultClusterScopedKinds,
	}

	data, _ := yaml.Marshal(&cfg)
	return string(data)
}

// GenerateFunctionVersionsFile generates a function-versions.yaml file.
func GenerateFunctionVersionsFile() string {
	data, _ := yaml.Marshal(DefaultFunctionVersions)
	return string(data)
}

// DefaultFunctionVersions defines the built-in recommended function versions.
// These can be overridden by external configuration.
var DefaultFunctionVersions = map[string]string{
	"function-go-templating":       "v0.12.1",
	"function-auto-ready":          "v0.6.5",
	"function-extra-resources":     "v0.3.0",
	"function-patch-and-transform": "v0.10.6",
}

// DefaultProviderMappings defines the built-in provider API group migrations.
var DefaultProviderMappings = map[string]string{
	"aws.upbound.io":           "aws.m.upbound.io",
	"azure.upbound.io":         "azure.m.upbound.io",
	"gcp.upbound.io":           "gcp.m.upbound.io",
	"kubernetes.crossplane.io": "kubernetes.m.crossplane.io",
}

// DefaultClusterScopedKinds lists the built-in cluster-scoped resource kinds.
var DefaultClusterScopedKinds = []string{
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
