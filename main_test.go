package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zufardhiyaulhaq/safekubectl/internal/config"
)

func TestRunEmptyArgs(t *testing.T) {
	executed := false
	runner := &Runner{
		stdin:  strings.NewReader(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		getCluster: func() string {
			return "test-cluster"
		},
		getContextNamespace: func() string { return "default" },
		executeKubectl: func(args []string) error {
			executed = true
			if len(args) != 0 {
				t.Errorf("expected empty args, got %v", args)
			}
			return nil
		},
		loadConfig: func() (*config.Config, error) {
			return config.DefaultConfig(), nil
		},
	}

	err := runner.Run([]string{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !executed {
		t.Error("expected kubectl to be executed")
	}
}

func TestRunSafeOperation(t *testing.T) {
	executed := false
	var executedArgs []string

	runner := &Runner{
		stdin:  strings.NewReader(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		getCluster: func() string {
			return "test-cluster"
		},
		getContextNamespace: func() string { return "default" },
		executeKubectl: func(args []string) error {
			executed = true
			executedArgs = args
			return nil
		},
		loadConfig: func() (*config.Config, error) {
			return config.DefaultConfig(), nil
		},
	}

	err := runner.Run([]string{"get", "pods"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !executed {
		t.Error("expected kubectl to be executed")
	}

	if len(executedArgs) != 2 || executedArgs[0] != "get" || executedArgs[1] != "pods" {
		t.Errorf("unexpected args: %v", executedArgs)
	}
}

func TestRunDangerousOperationConfirmed(t *testing.T) {
	executed := false
	var stdout bytes.Buffer

	runner := &Runner{
		stdin:  strings.NewReader("y\n"),
		stdout: &stdout,
		stderr: &bytes.Buffer{},
		getCluster: func() string {
			return "test-cluster"
		},
		getContextNamespace: func() string { return "default" },
		executeKubectl: func(args []string) error {
			executed = true
			return nil
		},
		loadConfig: func() (*config.Config, error) {
			cfg := config.DefaultConfig()
			cfg.Audit.Enabled = false
			return cfg, nil
		},
	}

	err := runner.Run([]string{"delete", "pod", "nginx"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !executed {
		t.Error("expected kubectl to be executed after confirmation")
	}

	output := stdout.String()
	if !strings.Contains(output, "DANGEROUS OPERATION DETECTED") {
		t.Error("expected warning to be displayed")
	}
}

func TestRunDangerousOperationDenied(t *testing.T) {
	executed := false
	var stdout bytes.Buffer

	runner := &Runner{
		stdin:  strings.NewReader("n\n"),
		stdout: &stdout,
		stderr: &bytes.Buffer{},
		getCluster: func() string {
			return "test-cluster"
		},
		getContextNamespace: func() string { return "default" },
		executeKubectl: func(args []string) error {
			executed = true
			return nil
		},
		loadConfig: func() (*config.Config, error) {
			cfg := config.DefaultConfig()
			cfg.Audit.Enabled = false
			return cfg, nil
		},
	}

	err := runner.Run([]string{"delete", "pod", "nginx"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if executed {
		t.Error("expected kubectl NOT to be executed after denial")
	}

	output := stdout.String()
	if !strings.Contains(output, "Operation aborted") {
		t.Error("expected abort message to be displayed")
	}
}

func TestRunWarnOnlyMode(t *testing.T) {
	executed := false
	var stdout bytes.Buffer

	runner := &Runner{
		stdin:  strings.NewReader(""),
		stdout: &stdout,
		stderr: &bytes.Buffer{},
		getCluster: func() string {
			return "test-cluster"
		},
		getContextNamespace: func() string { return "default" },
		executeKubectl: func(args []string) error {
			executed = true
			return nil
		},
		loadConfig: func() (*config.Config, error) {
			cfg := config.DefaultConfig()
			cfg.Mode = config.ModeWarnOnly
			cfg.Audit.Enabled = false
			return cfg, nil
		},
	}

	err := runner.Run([]string{"delete", "pod", "nginx"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !executed {
		t.Error("expected kubectl to be executed in warn-only mode")
	}

	output := stdout.String()
	if !strings.Contains(output, "DANGEROUS OPERATION DETECTED") {
		t.Error("expected warning to be displayed")
	}
	if !strings.Contains(output, "Proceeding with operation") {
		t.Error("expected proceeding message to be displayed")
	}
}

func TestRunWarnOnlyModeProtectedNamespace(t *testing.T) {
	executed := false
	var stdout bytes.Buffer

	runner := &Runner{
		stdin:  strings.NewReader("n\n"),
		stdout: &stdout,
		stderr: &bytes.Buffer{},
		getCluster: func() string {
			return "test-cluster"
		},
		getContextNamespace: func() string { return "default" },
		executeKubectl: func(args []string) error {
			executed = true
			return nil
		},
		loadConfig: func() (*config.Config, error) {
			cfg := config.DefaultConfig()
			cfg.Mode = config.ModeWarnOnly
			cfg.ProtectedNamespaces = []string{"production"}
			cfg.Audit.Enabled = false
			return cfg, nil
		},
	}

	// Protected namespace should require confirmation even in warn-only mode
	err := runner.Run([]string{"delete", "pod", "nginx", "-n", "production"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if executed {
		t.Error("expected kubectl NOT to be executed for protected namespace when denied")
	}
}

func TestRunConfigLoadError(t *testing.T) {
	runner := &Runner{
		stdin:  strings.NewReader(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		getCluster: func() string {
			return "test-cluster"
		},
		getContextNamespace: func() string { return "default" },
		executeKubectl: func(args []string) error {
			return nil
		},
		loadConfig: func() (*config.Config, error) {
			return nil, errors.New("config error")
		},
	}

	err := runner.Run([]string{"delete", "pod", "nginx"})
	if err == nil {
		t.Error("expected error when config load fails")
	}

	if !strings.Contains(err.Error(), "failed to load config") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRunKubectlError(t *testing.T) {
	runner := &Runner{
		stdin:  strings.NewReader(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		getCluster: func() string {
			return "test-cluster"
		},
		getContextNamespace: func() string { return "default" },
		executeKubectl: func(args []string) error {
			return errors.New("kubectl error")
		},
		loadConfig: func() (*config.Config, error) {
			return config.DefaultConfig(), nil
		},
	}

	err := runner.Run([]string{"get", "pods"})
	if err == nil {
		t.Error("expected error when kubectl fails")
	}

	if err.Error() != "kubectl error" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRunWithAuditEnabled(t *testing.T) {
	tmpDir := t.TempDir()
	executed := false

	runner := &Runner{
		stdin:  strings.NewReader("y\n"),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		getCluster: func() string {
			return "test-cluster"
		},
		getContextNamespace: func() string { return "default" },
		executeKubectl: func(args []string) error {
			executed = true
			return nil
		},
		loadConfig: func() (*config.Config, error) {
			cfg := config.DefaultConfig()
			cfg.Audit.Enabled = true
			cfg.Audit.Path = tmpDir + "/audit.log"
			return cfg, nil
		},
	}

	err := runner.Run([]string{"delete", "pod", "nginx"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !executed {
		t.Error("expected kubectl to be executed")
	}
}

func TestRunWithAuditEnabledDenied(t *testing.T) {
	tmpDir := t.TempDir()
	executed := false

	runner := &Runner{
		stdin:  strings.NewReader("n\n"),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		getCluster: func() string {
			return "test-cluster"
		},
		getContextNamespace: func() string { return "default" },
		executeKubectl: func(args []string) error {
			executed = true
			return nil
		},
		loadConfig: func() (*config.Config, error) {
			cfg := config.DefaultConfig()
			cfg.Audit.Enabled = true
			cfg.Audit.Path = tmpDir + "/audit.log"
			return cfg, nil
		},
	}

	err := runner.Run([]string{"delete", "pod", "nginx"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if executed {
		t.Error("expected kubectl NOT to be executed")
	}
}

func TestRunMultipleDangerousOperations(t *testing.T) {
	operations := []string{"delete", "apply", "patch", "edit", "drain", "exec", "cordon", "taint", "rollout"}

	for _, op := range operations {
		t.Run(op, func(t *testing.T) {
			var stdout bytes.Buffer

			runner := &Runner{
				stdin:  strings.NewReader("n\n"),
				stdout: &stdout,
				stderr: &bytes.Buffer{},
				getCluster: func() string {
					return "test-cluster"
				},
				getContextNamespace: func() string { return "default" },
				executeKubectl: func(args []string) error {
					return nil
				},
				loadConfig: func() (*config.Config, error) {
					cfg := config.DefaultConfig()
					cfg.Audit.Enabled = false
					return cfg, nil
				},
			}

			args := []string{op, "resource", "name"}
			runner.Run(args)

			output := stdout.String()
			if !strings.Contains(output, "DANGEROUS OPERATION DETECTED") {
				t.Errorf("operation %q should be flagged as dangerous", op)
			}
		})
	}
}

func TestGetCurrentCluster(t *testing.T) {
	// This test will actually call kubectl
	// If kubectl is not available, it should return "<unknown>"
	cluster := getCurrentCluster()
	if cluster == "" {
		t.Error("getCurrentCluster should not return empty string")
	}
}

func TestRunWithFileInput(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "deploy.yaml")
	content := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  namespace: istio-system`
	os.WriteFile(manifestPath, []byte(content), 0644)

	cfg := &config.Config{
		Mode:                config.ModeConfirm,
		DangerousOperations: []string{"apply"},
		ProtectedNamespaces: []string{"istio-system"},
		ProtectedClusters:   []string{},
	}

	var stdout bytes.Buffer
	stdin := strings.NewReader("n\n")

	runner := &Runner{
		stdin:               stdin,
		stdout:              &stdout,
		stderr:              &bytes.Buffer{},
		getCluster:          func() string { return "test-cluster" },
		getContextNamespace: func() string { return "default" },
		executeKubectl:      func(args []string) error { return nil },
		loadConfig:          func() (*config.Config, error) { return cfg, nil },
	}

	err := runner.Run([]string{"apply", "-f", manifestPath})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "istio-system") {
		t.Errorf("Expected 'istio-system' in output, got: %s", output)
	}
	if !strings.Contains(output, "Deployment/nginx") {
		t.Errorf("Expected 'Deployment/nginx' in output, got: %s", output)
	}
}

func TestIntegrationMultiDocYAML(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "multi.yaml")
	content := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  namespace: istio-system
---
apiVersion: v1
kind: Service
metadata:
  name: nginx-svc
  namespace: default`
	os.WriteFile(manifestPath, []byte(content), 0644)

	cfg := &config.Config{
		Mode:                config.ModeConfirm,
		DangerousOperations: []string{"apply"},
		ProtectedNamespaces: []string{"istio-system"},
		ProtectedClusters:   []string{},
	}

	var stdout bytes.Buffer
	stdin := strings.NewReader("n\n")

	runner := &Runner{
		stdin:               stdin,
		stdout:              &stdout,
		stderr:              &bytes.Buffer{},
		getCluster:          func() string { return "test" },
		getContextNamespace: func() string { return "default" },
		executeKubectl:      func(args []string) error { return nil },
		loadConfig:          func() (*config.Config, error) { return cfg, nil },
	}

	runner.Run([]string{"apply", "-f", manifestPath})

	output := stdout.String()
	if !strings.Contains(output, "Deployment/nginx") {
		t.Error("Expected Deployment/nginx")
	}
	if !strings.Contains(output, "Service/nginx-svc") {
		t.Error("Expected Service/nginx-svc")
	}
	if !strings.Contains(output, "istio-system") {
		t.Error("Expected istio-system namespace")
	}
}

func TestIntegrationDirectoryRecursive(t *testing.T) {
	dir := t.TempDir()

	// Root level file
	os.WriteFile(filepath.Join(dir, "root.yaml"), []byte(`apiVersion: v1
kind: Pod
metadata:
  name: root-pod`), 0644)

	// Nested file
	subdir := filepath.Join(dir, "nested")
	os.Mkdir(subdir, 0755)
	os.WriteFile(filepath.Join(subdir, "nested.yaml"), []byte(`apiVersion: v1
kind: Pod
metadata:
  name: nested-pod`), 0644)

	cfg := &config.Config{
		Mode:                config.ModeConfirm,
		DangerousOperations: []string{"apply"},
		ProtectedNamespaces: []string{},
		ProtectedClusters:   []string{},
	}

	// Test without -R (should only get root-pod)
	var stdout1 bytes.Buffer
	runner1 := &Runner{
		stdin:               strings.NewReader("n\n"),
		stdout:              &stdout1,
		stderr:              &bytes.Buffer{},
		getCluster:          func() string { return "test" },
		getContextNamespace: func() string { return "default" },
		executeKubectl:      func(args []string) error { return nil },
		loadConfig:          func() (*config.Config, error) { return cfg, nil },
	}

	runner1.Run([]string{"apply", "-f", dir})
	output1 := stdout1.String()
	if !strings.Contains(output1, "root-pod") {
		t.Error("Expected root-pod without -R")
	}
	if strings.Contains(output1, "nested-pod") {
		t.Error("Should not include nested-pod without -R")
	}

	// Test with -R (should get both)
	var stdout2 bytes.Buffer
	runner2 := &Runner{
		stdin:               strings.NewReader("n\n"),
		stdout:              &stdout2,
		stderr:              &bytes.Buffer{},
		getCluster:          func() string { return "test" },
		getContextNamespace: func() string { return "default" },
		executeKubectl:      func(args []string) error { return nil },
		loadConfig:          func() (*config.Config, error) { return cfg, nil },
	}

	runner2.Run([]string{"apply", "-f", dir, "-R"})
	output2 := stdout2.String()
	if !strings.Contains(output2, "root-pod") {
		t.Error("Expected root-pod with -R")
	}
	if !strings.Contains(output2, "nested-pod") {
		t.Error("Expected nested-pod with -R")
	}
}

func TestIntegrationFallbackNamespace(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "no-ns.yaml")
	content := `apiVersion: v1
kind: Pod
metadata:
  name: test-pod`
	os.WriteFile(manifestPath, []byte(content), 0644)

	cfg := &config.Config{
		Mode:                config.ModeConfirm,
		DangerousOperations: []string{"apply"},
		ProtectedNamespaces: []string{"my-namespace"},
		ProtectedClusters:   []string{},
	}

	var stdout bytes.Buffer
	stdin := strings.NewReader("n\n")

	runner := &Runner{
		stdin:               stdin,
		stdout:              &stdout,
		stderr:              &bytes.Buffer{},
		getCluster:          func() string { return "test" },
		getContextNamespace: func() string { return "my-namespace" },
		executeKubectl:      func(args []string) error { return nil },
		loadConfig:          func() (*config.Config, error) { return cfg, nil },
	}

	runner.Run([]string{"apply", "-f", manifestPath})

	output := stdout.String()
	if !strings.Contains(output, "my-namespace") {
		t.Error("Expected my-namespace (from context)")
	}
	if !strings.Contains(output, "protected namespace") {
		t.Error("Expected protected namespace warning")
	}
}

func TestRunNamespaceFromContext(t *testing.T) {
	// Bug: When no -n flag is provided, the warning should show the namespace
	// from kubectl context, not "default"
	var stdout bytes.Buffer

	runner := &Runner{
		stdin:  strings.NewReader("n\n"),
		stdout: &stdout,
		stderr: &bytes.Buffer{},
		getCluster: func() string {
			return "test-cluster"
		},
		getContextNamespace: func() string { return "kong-system" }, // Context namespace
		executeKubectl: func(args []string) error {
			return nil
		},
		loadConfig: func() (*config.Config, error) {
			cfg := config.DefaultConfig()
			cfg.Audit.Enabled = false
			return cfg, nil
		},
	}

	// No -n flag provided
	err := runner.Run([]string{"delete", "pod", "nginx"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	output := stdout.String()
	// Should show context namespace, not "default"
	if !strings.Contains(output, "kong-system") {
		t.Errorf("expected namespace 'kong-system' from context in output, got: %s", output)
	}
	if strings.Contains(output, "Namespace: default") {
		t.Errorf("should not show 'default' when context namespace is 'kong-system', got: %s", output)
	}
}

func TestRunNamespaceExplicitOverridesContext(t *testing.T) {
	// When -n flag is provided, it should override the context namespace
	var stdout bytes.Buffer

	runner := &Runner{
		stdin:  strings.NewReader("n\n"),
		stdout: &stdout,
		stderr: &bytes.Buffer{},
		getCluster: func() string {
			return "test-cluster"
		},
		getContextNamespace: func() string { return "kong-system" }, // Context namespace
		executeKubectl: func(args []string) error {
			return nil
		},
		loadConfig: func() (*config.Config, error) {
			cfg := config.DefaultConfig()
			cfg.Audit.Enabled = false
			return cfg, nil
		},
	}

	// Explicit -n flag should take precedence
	err := runner.Run([]string{"delete", "pod", "nginx", "-n", "production"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	output := stdout.String()
	// Should show explicit namespace, not context namespace
	if !strings.Contains(output, "production") {
		t.Errorf("expected namespace 'production' in output, got: %s", output)
	}
}

func TestIntegrationFileParseError(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "invalid.yaml")
	content := `invalid: yaml: content: [[[`
	os.WriteFile(manifestPath, []byte(content), 0644)

	cfg := &config.Config{
		Mode:                config.ModeConfirm,
		DangerousOperations: []string{"apply"},
	}

	runner := &Runner{
		stdin:               strings.NewReader(""),
		stdout:              &bytes.Buffer{},
		stderr:              &bytes.Buffer{},
		getCluster:          func() string { return "test" },
		getContextNamespace: func() string { return "default" },
		executeKubectl:      func(args []string) error { return nil },
		loadConfig:          func() (*config.Config, error) { return cfg, nil },
	}

	err := runner.Run([]string{"apply", "-f", manifestPath})
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}
