package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zufardhiyaulhaq/safekubectl/internal/checker"
	"github.com/zufardhiyaulhaq/safekubectl/internal/config"
)

func TestNew(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := New(cfg)

	if logger == nil {
		t.Fatal("New() returned nil")
	}

	if logger.config != cfg {
		t.Error("New() did not set config correctly")
	}
}

func TestLogDisabled(t *testing.T) {
	cfg := &config.Config{
		Audit: config.AuditConfig{
			Enabled: false,
			Path:    "/tmp/test-audit.log",
		},
	}

	logger := New(cfg)
	result := &checker.CheckResult{
		Operation: "delete",
		Resource:  "pod/nginx",
		Namespace: "default",
		Cluster:   "test-cluster",
	}

	err := logger.Log(result, []string{"delete", "pod", "nginx"}, true, true)
	if err != nil {
		t.Errorf("Log() with disabled audit returned error: %v", err)
	}

	// File should not exist since audit is disabled
	if _, err := os.Stat(cfg.Audit.Path); !os.IsNotExist(err) {
		os.Remove(cfg.Audit.Path)
		t.Error("Log() created file when audit is disabled")
	}
}

func TestLogEnabled(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	cfg := &config.Config{
		Audit: config.AuditConfig{
			Enabled: true,
			Path:    logPath,
		},
	}

	logger := New(cfg)
	result := &checker.CheckResult{
		Operation: "delete",
		Resource:  "pod/nginx",
		Namespace: "production",
		Cluster:   "prod-cluster",
	}

	err := logger.Log(result, []string{"delete", "pod", "nginx", "-n", "production"}, true, true)
	if err != nil {
		t.Fatalf("Log() returned error: %v", err)
	}

	// Read and verify log content
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	logContent := string(content)

	// Verify log entry contains expected fields
	expectedParts := []string{
		"EXECUTED",
		"operation=delete",
		"resource=pod/nginx",
		"namespace=production",
		"cluster=prod-cluster",
		"confirmed=true",
		"command=\"delete pod nginx -n production\"",
	}

	for _, part := range expectedParts {
		if !strings.Contains(logContent, part) {
			t.Errorf("log entry missing %q, got:\n%s", part, logContent)
		}
	}
}

func TestLogDenied(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	cfg := &config.Config{
		Audit: config.AuditConfig{
			Enabled: true,
			Path:    logPath,
		},
	}

	logger := New(cfg)
	result := &checker.CheckResult{
		Operation: "delete",
		Resource:  "pod/nginx",
		Namespace: "production",
		Cluster:   "prod-cluster",
	}

	err := logger.Log(result, []string{"delete", "pod", "nginx"}, false, false)
	if err != nil {
		t.Fatalf("Log() returned error: %v", err)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	logContent := string(content)

	if !strings.Contains(logContent, "DENIED") {
		t.Errorf("expected log to contain 'DENIED', got:\n%s", logContent)
	}

	if !strings.Contains(logContent, "confirmed=false") {
		t.Errorf("expected log to contain 'confirmed=false', got:\n%s", logContent)
	}
}

func TestLogAppendsToExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	cfg := &config.Config{
		Audit: config.AuditConfig{
			Enabled: true,
			Path:    logPath,
		},
	}

	logger := New(cfg)
	result := &checker.CheckResult{
		Operation: "delete",
		Resource:  "pod/nginx",
		Namespace: "default",
		Cluster:   "test-cluster",
	}

	// Write first entry
	if err := logger.Log(result, []string{"delete", "pod", "nginx"}, true, true); err != nil {
		t.Fatalf("first Log() failed: %v", err)
	}

	// Write second entry
	result2 := &checker.CheckResult{
		Operation: "apply",
		Resource:  "deployment/web",
		Namespace: "staging",
		Cluster:   "test-cluster",
	}

	if err := logger.Log(result2, []string{"apply", "-f", "deploy.yaml"}, true, true); err != nil {
		t.Fatalf("second Log() failed: %v", err)
	}

	// Read and verify both entries exist
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	logContent := string(content)
	lines := strings.Split(strings.TrimSpace(logContent), "\n")

	if len(lines) != 2 {
		t.Errorf("expected 2 log entries, got %d:\n%s", len(lines), logContent)
	}

	if !strings.Contains(lines[0], "operation=delete") {
		t.Errorf("first entry should contain delete operation")
	}

	if !strings.Contains(lines[1], "operation=apply") {
		t.Errorf("second entry should contain apply operation")
	}
}

func TestLogCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "nested", "dir", "audit.log")

	cfg := &config.Config{
		Audit: config.AuditConfig{
			Enabled: true,
			Path:    logPath,
		},
	}

	logger := New(cfg)
	result := &checker.CheckResult{
		Operation: "delete",
		Resource:  "pod/nginx",
		Namespace: "default",
		Cluster:   "test-cluster",
	}

	err := logger.Log(result, []string{"delete", "pod", "nginx"}, true, true)
	if err != nil {
		t.Fatalf("Log() failed to create nested directory: %v", err)
	}

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("Log() did not create log file in nested directory")
	}
}

func TestLogTimestampFormat(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	cfg := &config.Config{
		Audit: config.AuditConfig{
			Enabled: true,
			Path:    logPath,
		},
	}

	logger := New(cfg)
	result := &checker.CheckResult{
		Operation: "delete",
		Resource:  "pod/nginx",
		Namespace: "default",
		Cluster:   "test-cluster",
	}

	if err := logger.Log(result, []string{"delete", "pod", "nginx"}, true, true); err != nil {
		t.Fatalf("Log() failed: %v", err)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	logContent := string(content)

	// Check for RFC3339 timestamp format [2006-01-02T15:04:05...]
	if !strings.HasPrefix(logContent, "[2") {
		t.Errorf("log entry should start with timestamp, got:\n%s", logContent)
	}

	if !strings.Contains(logContent, "T") {
		t.Errorf("timestamp should be in RFC3339 format, got:\n%s", logContent)
	}
}

func TestLogWithSpecialCharactersInCommand(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	cfg := &config.Config{
		Audit: config.AuditConfig{
			Enabled: true,
			Path:    logPath,
		},
	}

	logger := New(cfg)
	result := &checker.CheckResult{
		Operation: "patch",
		Resource:  "deployment/nginx",
		Namespace: "default",
		Cluster:   "test-cluster",
	}

	// Command with special characters (JSON patch)
	args := []string{"patch", "deployment", "nginx", "-p", `{"spec":{"replicas":3}}`}

	if err := logger.Log(result, args, true, true); err != nil {
		t.Fatalf("Log() failed: %v", err)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	logContent := string(content)

	// Verify command is logged (JSON characters are preserved as-is)
	if !strings.Contains(logContent, `command="patch deployment nginx -p {"spec":{"replicas":3}}"`) {
		t.Errorf("command with special characters not logged correctly:\n%s", logContent)
	}
}

func TestLogInvalidPath(t *testing.T) {
	cfg := &config.Config{
		Audit: config.AuditConfig{
			Enabled: true,
			Path:    "/nonexistent/readonly/path/audit.log",
		},
	}

	logger := New(cfg)
	result := &checker.CheckResult{
		Operation: "delete",
		Resource:  "pod/nginx",
		Namespace: "default",
		Cluster:   "test-cluster",
	}

	err := logger.Log(result, []string{"delete", "pod", "nginx"}, true, true)
	// Should return error for invalid path
	if err == nil {
		t.Error("expected error for invalid path, got nil")
	}
}
