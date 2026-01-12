package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Mode != ModeConfirm {
		t.Errorf("expected default mode to be %q, got %q", ModeConfirm, cfg.Mode)
	}

	expectedOps := []string{
		"delete", "apply", "patch", "edit", "update",
		"rollout", "drain", "exec", "cordon", "taint",
	}

	if len(cfg.DangerousOperations) != len(expectedOps) {
		t.Errorf("expected %d dangerous operations, got %d", len(expectedOps), len(cfg.DangerousOperations))
	}

	for i, op := range expectedOps {
		if cfg.DangerousOperations[i] != op {
			t.Errorf("expected dangerous operation %d to be %q, got %q", i, op, cfg.DangerousOperations[i])
		}
	}

	if len(cfg.ProtectedNamespaces) != 1 || cfg.ProtectedNamespaces[0] != "kube-system" {
		t.Errorf("expected protected namespaces to be [kube-system], got %v", cfg.ProtectedNamespaces)
	}

	if len(cfg.ProtectedClusters) != 0 {
		t.Errorf("expected no protected clusters by default, got %v", cfg.ProtectedClusters)
	}

	if cfg.Audit.Enabled {
		t.Error("expected audit to be disabled by default")
	}
}

func TestIsDangerousOperation(t *testing.T) {
	cfg := DefaultConfig()

	tests := []struct {
		operation string
		expected  bool
	}{
		{"delete", true},
		{"apply", true},
		{"patch", true},
		{"edit", true},
		{"update", true},
		{"rollout", true},
		{"drain", true},
		{"exec", true},
		{"cordon", true},
		{"taint", true},
		{"get", false},
		{"describe", false},
		{"logs", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.operation, func(t *testing.T) {
			result := cfg.IsDangerousOperation(tt.operation)
			if result != tt.expected {
				t.Errorf("IsDangerousOperation(%q) = %v, expected %v", tt.operation, result, tt.expected)
			}
		})
	}
}

func TestIsProtectedNamespace(t *testing.T) {
	cfg := &Config{
		ProtectedNamespaces: []string{"kube-system", "production", "prod"},
	}

	tests := []struct {
		namespace string
		expected  bool
	}{
		{"kube-system", true},
		{"production", true},
		{"prod", true},
		{"default", false},
		{"staging", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.namespace, func(t *testing.T) {
			result := cfg.IsProtectedNamespace(tt.namespace)
			if result != tt.expected {
				t.Errorf("IsProtectedNamespace(%q) = %v, expected %v", tt.namespace, result, tt.expected)
			}
		})
	}
}

func TestIsProtectedCluster(t *testing.T) {
	cfg := &Config{
		ProtectedClusters: []string{"prod-us-east-1", "prod-eu-west-1"},
	}

	tests := []struct {
		cluster  string
		expected bool
	}{
		{"prod-us-east-1", true},
		{"prod-eu-west-1", true},
		{"staging-us-east-1", false},
		{"dev-cluster", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.cluster, func(t *testing.T) {
			result := cfg.IsProtectedCluster(tt.cluster)
			if result != tt.expected {
				t.Errorf("IsProtectedCluster(%q) = %v, expected %v", tt.cluster, result, tt.expected)
			}
		})
	}
}

func TestRequiresConfirmation(t *testing.T) {
	tests := []struct {
		name      string
		mode      Mode
		namespace string
		cluster   string
		protected []string
		clusters  []string
		expected  bool
	}{
		{
			name:      "confirm mode always requires confirmation",
			mode:      ModeConfirm,
			namespace: "default",
			cluster:   "dev",
			protected: []string{},
			clusters:  []string{},
			expected:  true,
		},
		{
			name:      "warn-only mode with non-protected resources",
			mode:      ModeWarnOnly,
			namespace: "default",
			cluster:   "dev",
			protected: []string{"production"},
			clusters:  []string{"prod-cluster"},
			expected:  false,
		},
		{
			name:      "warn-only mode with protected namespace",
			mode:      ModeWarnOnly,
			namespace: "production",
			cluster:   "dev",
			protected: []string{"production"},
			clusters:  []string{},
			expected:  true,
		},
		{
			name:      "warn-only mode with protected cluster",
			mode:      ModeWarnOnly,
			namespace: "default",
			cluster:   "prod-cluster",
			protected: []string{},
			clusters:  []string{"prod-cluster"},
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Mode:                tt.mode,
				ProtectedNamespaces: tt.protected,
				ProtectedClusters:   tt.clusters,
			}
			result := cfg.RequiresConfirmation(tt.namespace, tt.cluster)
			if result != tt.expected {
				t.Errorf("RequiresConfirmation(%q, %q) = %v, expected %v", tt.namespace, tt.cluster, result, tt.expected)
			}
		})
	}
}

func TestExpandPath(t *testing.T) {
	homeDir, _ := os.UserHomeDir()

	tests := []struct {
		input    string
		expected string
	}{
		{"~/test/path", filepath.Join(homeDir, "test/path")},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"~", homeDir},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := expandPath(tt.input)
			if result != tt.expected {
				t.Errorf("expandPath(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	// Test loading with non-existent config file (should return defaults)
	t.Run("non-existent config returns defaults", func(t *testing.T) {
		// Set env to non-existent path
		os.Setenv("SAFEKUBECTL_CONFIG", "/non/existent/path/config.yaml")
		defer os.Unsetenv("SAFEKUBECTL_CONFIG")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.Mode != ModeConfirm {
			t.Errorf("expected default mode, got %q", cfg.Mode)
		}
	})

	// Test loading with valid config file
	t.Run("valid config file", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		configContent := `
mode: warn-only
dangerousOperations:
  - delete
  - apply
protectedNamespaces:
  - production
protectedClusters:
  - prod-cluster
audit:
  enabled: true
  path: /var/log/safekubectl.log
`
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("failed to write config file: %v", err)
		}

		os.Setenv("SAFEKUBECTL_CONFIG", configPath)
		defer os.Unsetenv("SAFEKUBECTL_CONFIG")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.Mode != ModeWarnOnly {
			t.Errorf("expected mode %q, got %q", ModeWarnOnly, cfg.Mode)
		}

		if len(cfg.DangerousOperations) != 2 {
			t.Errorf("expected 2 dangerous operations, got %d", len(cfg.DangerousOperations))
		}

		if len(cfg.ProtectedNamespaces) != 1 || cfg.ProtectedNamespaces[0] != "production" {
			t.Errorf("expected protected namespaces [production], got %v", cfg.ProtectedNamespaces)
		}

		if !cfg.Audit.Enabled {
			t.Error("expected audit to be enabled")
		}
	})

	// Test loading with invalid YAML
	t.Run("invalid yaml returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		if err := os.WriteFile(configPath, []byte("invalid: yaml: content:"), 0644); err != nil {
			t.Fatalf("failed to write config file: %v", err)
		}

		os.Setenv("SAFEKUBECTL_CONFIG", configPath)
		defer os.Unsetenv("SAFEKUBECTL_CONFIG")

		_, err := Load()
		if err == nil {
			t.Error("expected error for invalid YAML, got nil")
		}
	})
}

func TestGetConfigPath(t *testing.T) {
	t.Run("env var takes precedence", func(t *testing.T) {
		os.Setenv("SAFEKUBECTL_CONFIG", "/custom/path/config.yaml")
		defer os.Unsetenv("SAFEKUBECTL_CONFIG")

		path := getConfigPath()
		if path != "/custom/path/config.yaml" {
			t.Errorf("expected /custom/path/config.yaml, got %q", path)
		}
	})

	t.Run("default path when no env var", func(t *testing.T) {
		os.Unsetenv("SAFEKUBECTL_CONFIG")

		homeDir, _ := os.UserHomeDir()
		expected := filepath.Join(homeDir, ".safekubectl", "config.yaml")

		path := getConfigPath()
		if path != expected {
			t.Errorf("expected %q, got %q", expected, path)
		}
	})
}
