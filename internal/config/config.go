package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Mode represents the confirmation mode
type Mode string

const (
	ModeConfirm  Mode = "confirm"
	ModeWarnOnly Mode = "warn-only"
)

// AuditConfig holds audit logging configuration
type AuditConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

// Config holds the safekubectl configuration
type Config struct {
	Mode                Mode        `yaml:"mode"`
	DangerousOperations []string    `yaml:"dangerousOperations"`
	ProtectedNamespaces []string    `yaml:"protectedNamespaces"`
	ProtectedClusters   []string    `yaml:"protectedClusters"`
	Audit               AuditConfig `yaml:"audit"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	return &Config{
		Mode: ModeConfirm,
		DangerousOperations: []string{
			"delete",
			"apply",
			"patch",
			"edit",
			"update",
			"rollout",
			"drain",
			"exec",
			"cordon",
			"taint",
		},
		ProtectedNamespaces: []string{
			"kube-system",
		},
		ProtectedClusters: []string{},
		Audit: AuditConfig{
			Enabled: false,
			Path:    filepath.Join(homeDir, ".safekubectl", "audit.log"),
		},
	}
}

// getConfigPath returns the config file path
func getConfigPath() string {
	// Check environment variable first
	if envPath := os.Getenv("SAFEKUBECTL_CONFIG"); envPath != "" {
		return envPath
	}

	// Default to ~/.safekubectl/config.yaml
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".safekubectl", "config.yaml")
}

// Load loads the configuration from file or returns defaults
func Load() (*Config, error) {
	config := DefaultConfig()

	configPath := getConfigPath()
	if configPath == "" {
		return config, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Config file doesn't exist, use defaults
			return config, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, err
	}

	// Expand ~ in audit path
	if config.Audit.Path != "" {
		config.Audit.Path = expandPath(config.Audit.Path)
	}

	return config, nil
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(homeDir, path[1:])
	}
	return path
}

// IsDangerousOperation checks if an operation is in the dangerous list
func (c *Config) IsDangerousOperation(operation string) bool {
	for _, op := range c.DangerousOperations {
		if op == operation {
			return true
		}
	}
	return false
}

// IsProtectedNamespace checks if a namespace is protected
func (c *Config) IsProtectedNamespace(namespace string) bool {
	for _, ns := range c.ProtectedNamespaces {
		if ns == namespace {
			return true
		}
	}
	return false
}

// IsProtectedCluster checks if a cluster is protected
func (c *Config) IsProtectedCluster(cluster string) bool {
	for _, cl := range c.ProtectedClusters {
		if cl == cluster {
			return true
		}
	}
	return false
}

// RequiresConfirmation returns true if confirm mode or protected resource
func (c *Config) RequiresConfirmation(namespace, cluster string) bool {
	if c.Mode == ModeConfirm {
		return true
	}
	// Even in warn-only mode, protected resources require confirmation
	return c.IsProtectedNamespace(namespace) || c.IsProtectedCluster(cluster)
}
