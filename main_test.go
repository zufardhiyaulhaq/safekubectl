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
		getContextNamespace: func(ctx string) string { return "default" },
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
		getContextNamespace: func(ctx string) string { return "default" },
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
		getContextNamespace: func(ctx string) string { return "default" },
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
		getContextNamespace: func(ctx string) string { return "default" },
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
		getContextNamespace: func(ctx string) string { return "default" },
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
		getContextNamespace: func(ctx string) string { return "default" },
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
		getContextNamespace: func(ctx string) string { return "default" },
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
		getContextNamespace: func(ctx string) string { return "default" },
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
		getContextNamespace: func(ctx string) string { return "default" },
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
		getContextNamespace: func(ctx string) string { return "default" },
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
				getContextNamespace: func(ctx string) string { return "default" },
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
		getContextNamespace: func(ctx string) string { return "default" },
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
		getContextNamespace: func(ctx string) string { return "default" },
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
		getContextNamespace: func(ctx string) string { return "default" },
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
		getContextNamespace: func(ctx string) string { return "default" },
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
		getContextNamespace: func(ctx string) string { return "my-namespace" },
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
		getContextNamespace: func(ctx string) string { return "kong-system" }, // Context namespace
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
		getContextNamespace: func(ctx string) string { return "kong-system" }, // Context namespace
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

func TestRunWithFileInputAuditLogging(t *testing.T) {
	// Test: File-based commands (apply -f) should write to audit log
	tmpDir := t.TempDir()
	auditPath := filepath.Join(tmpDir, "audit.log")

	manifestPath := filepath.Join(tmpDir, "deploy.yaml")
	content := `apiVersion: v1
kind: Pod
metadata:
  name: nginx
  namespace: test-ns`
	os.WriteFile(manifestPath, []byte(content), 0644)

	cfg := &config.Config{
		Mode:                config.ModeConfirm,
		DangerousOperations: []string{"apply"},
		Audit: config.AuditConfig{
			Enabled: true,
			Path:    auditPath,
		},
	}

	runner := &Runner{
		stdin:               strings.NewReader("y\n"), // Confirm
		stdout:              &bytes.Buffer{},
		stderr:              &bytes.Buffer{},
		getCluster:          func() string { return "test-cluster" },
		getContextNamespace: func(ctx string) string { return "default" },
		executeKubectl:      func(args []string) error { return nil },
		loadConfig:          func() (*config.Config, error) { return cfg, nil },
	}

	err := runner.Run([]string{"apply", "-f", manifestPath})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Check if audit log was written
	auditContent, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("Audit log should exist: %v", err)
	}

	if len(auditContent) == 0 {
		t.Error("Audit log should not be empty")
	}

	content2 := string(auditContent)
	if !strings.Contains(content2, "EXECUTED") {
		t.Errorf("Audit log should contain EXECUTED, got: %s", content2)
	}
	if !strings.Contains(content2, "apply") {
		t.Errorf("Audit log should contain operation 'apply', got: %s", content2)
	}
	if !strings.Contains(content2, "Pod/nginx") {
		t.Errorf("Audit log should contain resource 'Pod/nginx', got: %s", content2)
	}
}

func TestRunWithFileInputAuditLoggingDenied(t *testing.T) {
	// Test: Denied file-based commands should also be logged
	tmpDir := t.TempDir()
	auditPath := filepath.Join(tmpDir, "audit.log")

	manifestPath := filepath.Join(tmpDir, "deploy.yaml")
	content := `apiVersion: v1
kind: Pod
metadata:
  name: nginx`
	os.WriteFile(manifestPath, []byte(content), 0644)

	cfg := &config.Config{
		Mode:                config.ModeConfirm,
		DangerousOperations: []string{"apply"},
		Audit: config.AuditConfig{
			Enabled: true,
			Path:    auditPath,
		},
	}

	runner := &Runner{
		stdin:               strings.NewReader("n\n"), // Deny
		stdout:              &bytes.Buffer{},
		stderr:              &bytes.Buffer{},
		getCluster:          func() string { return "test-cluster" },
		getContextNamespace: func(ctx string) string { return "default" },
		executeKubectl:      func(args []string) error { return nil },
		loadConfig:          func() (*config.Config, error) { return cfg, nil },
	}

	err := runner.Run([]string{"apply", "-f", manifestPath})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Check if audit log was written
	auditContent, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("Audit log should exist: %v", err)
	}

	content2 := string(auditContent)
	if !strings.Contains(content2, "DENIED") {
		t.Errorf("Audit log should contain DENIED for denied operation, got: %s", content2)
	}
}

func TestContextFlagNamespaceResolution(t *testing.T) {
	// Test: When --context is provided, namespace should come from that context
	var stdout bytes.Buffer

	runner := &Runner{
		stdin:  strings.NewReader("n\n"),
		stdout: &stdout,
		stderr: &bytes.Buffer{},
		getCluster: func() string {
			return "test-cluster"
		},
		getContextNamespace: func(ctx string) string {
			// Return different namespace based on context
			if ctx == "other-cluster" {
				return "other-ns"
			}
			return "current-ns"
		},
		executeKubectl: func(args []string) error { return nil },
		loadConfig: func() (*config.Config, error) {
			cfg := config.DefaultConfig()
			cfg.Audit.Enabled = false
			return cfg, nil
		},
	}

	// User specifies --context, should use that context's namespace
	runner.Run([]string{"--context", "other-cluster", "delete", "pod", "nginx"})

	output := stdout.String()
	// Should show namespace from "other-cluster" context
	if !strings.Contains(output, "other-ns") {
		t.Errorf("Expected namespace 'other-ns' from specified context, got: %s", output)
	}
}

func TestRunDryRunSkipsWarning(t *testing.T) {
	// Dry-run commands should NOT trigger warnings
	executed := false
	var stdout bytes.Buffer

	runner := &Runner{
		stdin:  strings.NewReader(""),
		stdout: &stdout,
		stderr: &bytes.Buffer{},
		getCluster: func() string {
			return "test-cluster"
		},
		getContextNamespace: func(ctx string) string { return "default" },
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

	// --dry-run should not trigger warning
	err := runner.Run([]string{"delete", "pod", "nginx", "--dry-run=client"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !executed {
		t.Error("expected kubectl to be executed for dry-run")
	}

	output := stdout.String()
	if strings.Contains(output, "DANGEROUS OPERATION DETECTED") {
		t.Error("dry-run should not trigger dangerous operation warning")
	}
}

func TestRunDryRunFileInputSkipsWarning(t *testing.T) {
	// Bug: File-based commands (apply -f) with --dry-run should also skip warnings
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "deploy.yaml")
	content := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  namespace: kube-system`
	os.WriteFile(manifestPath, []byte(content), 0644)

	executed := false
	var stdout bytes.Buffer

	runner := &Runner{
		stdin:  strings.NewReader(""), // No confirmation input needed
		stdout: &stdout,
		stderr: &bytes.Buffer{},
		getCluster: func() string {
			return "test-cluster"
		},
		getContextNamespace: func(ctx string) string { return "default" },
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

	// apply -f with --dry-run should NOT trigger warning
	err := runner.Run([]string{"apply", "-f", manifestPath, "--dry-run=client"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !executed {
		t.Error("expected kubectl to be executed for dry-run")
	}

	output := stdout.String()
	if strings.Contains(output, "DANGEROUS OPERATION DETECTED") {
		t.Error("dry-run file-based command should not trigger dangerous operation warning")
	}
}

func TestRunAllNamespacesRequiresConfirmation(t *testing.T) {
	// --all-namespaces should ALWAYS require confirmation, even in warn-only mode
	executed := false
	var stdout bytes.Buffer

	runner := &Runner{
		stdin:  strings.NewReader("n\n"), // Deny
		stdout: &stdout,
		stderr: &bytes.Buffer{},
		getCluster: func() string {
			return "test-cluster"
		},
		getContextNamespace: func(ctx string) string { return "default" },
		executeKubectl: func(args []string) error {
			executed = true
			return nil
		},
		loadConfig: func() (*config.Config, error) {
			cfg := config.DefaultConfig()
			cfg.Mode = config.ModeWarnOnly // Even in warn-only mode
			cfg.Audit.Enabled = false
			return cfg, nil
		},
	}

	err := runner.Run([]string{"delete", "pods", "--all", "-A"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if executed {
		t.Error("expected kubectl NOT to be executed when all-namespaces denied")
	}

	output := stdout.String()
	if !strings.Contains(output, "ALL NAMESPACES") {
		t.Errorf("expected warning about ALL NAMESPACES, got: %s", output)
	}
}

func TestRunNodeScopedNoNamespace(t *testing.T) {
	// Node-scoped operations (drain, cordon) should not show namespace
	var stdout bytes.Buffer

	runner := &Runner{
		stdin:  strings.NewReader("n\n"),
		stdout: &stdout,
		stderr: &bytes.Buffer{},
		getCluster: func() string {
			return "test-cluster"
		},
		getContextNamespace: func(ctx string) string { return "some-namespace" },
		executeKubectl: func(args []string) error {
			return nil
		},
		loadConfig: func() (*config.Config, error) {
			cfg := config.DefaultConfig()
			cfg.Audit.Enabled = false
			return cfg, nil
		},
	}

	err := runner.Run([]string{"drain", "node-1", "--ignore-daemonsets"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	output := stdout.String()
	// Should not show "Namespace:" line for node-scoped operations
	if strings.Contains(output, "Namespace:") {
		t.Errorf("node-scoped operations should not show namespace, got: %s", output)
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
		getContextNamespace: func(ctx string) string { return "default" },
		executeKubectl:      func(args []string) error { return nil },
		loadConfig:          func() (*config.Config, error) { return cfg, nil },
	}

	err := runner.Run([]string{"apply", "-f", manifestPath})
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}
