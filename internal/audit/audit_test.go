package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zufardhiyaulhaq/safekubectl/internal/checker"
	"github.com/zufardhiyaulhaq/safekubectl/internal/config"
	"github.com/zufardhiyaulhaq/safekubectl/internal/manifest"
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
		Resources: []string{"pod/nginx"},
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
		Resources: []string{"pod/nginx"},
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
		"resources=[pod/nginx]",
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
		Resources: []string{"pod/nginx"},
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
		Resources: []string{"pod/nginx"},
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
		Resources: []string{"deployment/web"},
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
		Resources: []string{"pod/nginx"},
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
		Resources: []string{"pod/nginx"},
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
		Resources: []string{"deployment/nginx"},
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
		Resources: []string{"pod/nginx"},
		Namespace: "default",
		Cluster:   "test-cluster",
	}

	err := logger.Log(result, []string{"delete", "pod", "nginx"}, true, true)
	// Should return error for invalid path
	if err == nil {
		t.Error("expected error for invalid path, got nil")
	}
}

func TestLogResourcesEnabled(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	cfg := &config.Config{
		Audit: config.AuditConfig{
			Enabled: true,
			Path:    logPath,
		},
	}

	logger := New(cfg)
	result := &checker.ResourceCheckResult{
		Operation: "apply",
		Cluster:   "prod-cluster",
		Resources: []manifest.Resource{
			{Kind: "Deployment", Name: "nginx", Namespace: "production"},
			{Kind: "Service", Name: "nginx-svc", Namespace: "production"},
		},
		Reasons: []string{"dangerous operation: apply"},
	}

	err := logger.LogResources(result, []string{"apply", "-f", "deploy.yaml"}, true, true)
	if err != nil {
		t.Fatalf("LogResources() returned error: %v", err)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	logContent := string(content)

	// Verify log entry contains expected fields
	expectedParts := []string{
		"EXECUTED",
		"operation=apply",
		"cluster=prod-cluster",
		"Deployment/nginx@production",
		"Service/nginx-svc@production",
		"confirmed=true",
	}

	for _, part := range expectedParts {
		if !strings.Contains(logContent, part) {
			t.Errorf("log entry missing %q, got:\n%s", part, logContent)
		}
	}
}

func TestLogResourcesDisabled(t *testing.T) {
	cfg := &config.Config{
		Audit: config.AuditConfig{
			Enabled: false,
			Path:    "/tmp/test-audit.log",
		},
	}

	logger := New(cfg)
	result := &checker.ResourceCheckResult{
		Operation: "apply",
		Cluster:   "test-cluster",
		Resources: []manifest.Resource{
			{Kind: "Pod", Name: "test", Namespace: "default"},
		},
	}

	err := logger.LogResources(result, []string{"apply", "-f", "pod.yaml"}, true, true)
	if err != nil {
		t.Errorf("LogResources() with disabled audit returned error: %v", err)
	}

	// File should not exist since audit is disabled
	if _, err := os.Stat(cfg.Audit.Path); !os.IsNotExist(err) {
		os.Remove(cfg.Audit.Path)
		t.Error("LogResources() created file when audit is disabled")
	}
}

func TestLogResourcesDenied(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	cfg := &config.Config{
		Audit: config.AuditConfig{
			Enabled: true,
			Path:    logPath,
		},
	}

	logger := New(cfg)
	result := &checker.ResourceCheckResult{
		Operation: "delete",
		Cluster:   "prod-cluster",
		Resources: []manifest.Resource{
			{Kind: "Deployment", Name: "critical", Namespace: "kube-system"},
		},
	}

	err := logger.LogResources(result, []string{"delete", "-f", "deploy.yaml"}, false, false)
	if err != nil {
		t.Fatalf("LogResources() returned error: %v", err)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	logContent := string(content)

	if !strings.Contains(logContent, "DENIED") {
		t.Errorf("expected log to contain 'DENIED', got:\n%s", logContent)
	}
}

func TestLogResourcesEmptyNamespace(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	cfg := &config.Config{
		Audit: config.AuditConfig{
			Enabled: true,
			Path:    logPath,
		},
	}

	logger := New(cfg)
	result := &checker.ResourceCheckResult{
		Operation: "apply",
		Cluster:   "test-cluster",
		Resources: []manifest.Resource{
			{Kind: "ClusterRole", Name: "admin", Namespace: ""}, // Cluster-scoped
		},
	}

	err := logger.LogResources(result, []string{"apply", "-f", "clusterrole.yaml"}, true, true)
	if err != nil {
		t.Fatalf("LogResources() returned error: %v", err)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	logContent := string(content)

	// Empty namespace should default to "default" in log
	if !strings.Contains(logContent, "ClusterRole/admin@default") {
		t.Errorf("expected empty namespace to show as 'default', got:\n%s", logContent)
	}
}

func TestLogMultipleResources(t *testing.T) {
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
		Resources: []string{"secret/cert-a", "secret/cert-b"},
		Namespace: "istio-system",
		Cluster:   "prod-cluster",
	}

	err := logger.Log(result, []string{"delete", "secret", "cert-a", "cert-b"}, true, true)
	if err != nil {
		t.Fatalf("Log() returned error: %v", err)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "resources=[secret/cert-a,secret/cert-b]") {
		t.Errorf("log entry missing resources list, got:\n%s", string(content))
	}
}

func TestFormatText(t *testing.T) {
	e := Entry{
		Timestamp: "2026-06-14T10:38:14Z",
		Status:    "EXECUTED",
		Operation: "delete",
		Resources: []string{"secret/cert-a", "secret/cert-b"},
		Namespace: "istio-system",
		Cluster:   "prod-cluster",
		Confirmed: true,
		Command:   "delete secret cert-a cert-b -n istio-system",
	}

	got := formatText(e)
	want := `[2026-06-14T10:38:14Z] EXECUTED | operation=delete resources=[secret/cert-a,secret/cert-b] namespace=istio-system cluster=prod-cluster confirmed=true command="delete secret cert-a cert-b -n istio-system"`

	if got != want {
		t.Errorf("formatText():\n got: %s\nwant: %s", got, want)
	}
}

func TestFormatTextEmptyNamespace(t *testing.T) {
	e := Entry{
		Timestamp: "2026-06-14T10:38:14Z",
		Status:    "EXECUTED",
		Operation: "apply",
		Resources: []string{"Deployment/nginx@istio-system"},
		Namespace: "",
		Cluster:   "prod-cluster",
		Confirmed: true,
		Command:   "apply -f deploy.yaml",
	}

	got := formatText(e)
	if !strings.Contains(got, "resources=[Deployment/nginx@istio-system] namespace= cluster=prod-cluster") {
		t.Errorf("formatText() with empty namespace, got: %s", got)
	}
}

func TestFormatTextPreservesCommandQuotes(t *testing.T) {
	e := Entry{
		Timestamp: "2026-06-14T10:38:14Z",
		Status:    "EXECUTED",
		Operation: "patch",
		Resources: []string{"deployment/nginx"},
		Namespace: "default",
		Cluster:   "test-cluster",
		Confirmed: true,
		Command:   `patch deployment nginx -p {"spec":{"replicas":3}}`,
	}

	got := formatText(e)
	if !strings.Contains(got, `command="patch deployment nginx -p {"spec":{"replicas":3}}"`) {
		t.Errorf("formatText() must preserve unescaped quotes, got: %s", got)
	}
}

func TestFormatJSON(t *testing.T) {
	e := Entry{
		Timestamp: "2026-06-14T10:38:14Z",
		Status:    "DENIED",
		Operation: "delete",
		Resources: []string{"secret/cert-a", "secret/cert-b"},
		Namespace: "istio-system",
		Cluster:   "prod-cluster",
		Confirmed: false,
		Command:   "delete secret cert-a cert-b -n istio-system",
	}

	got, err := formatJSON(e)
	if err != nil {
		t.Fatalf("formatJSON() returned error: %v", err)
	}

	if strings.Contains(got, "\n") {
		t.Errorf("formatJSON() must not contain a newline, got: %s", got)
	}

	var decoded Entry
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("formatJSON() produced invalid JSON: %v\n%s", err, got)
	}

	if decoded.Status != "DENIED" {
		t.Errorf("status: got %q, want %q", decoded.Status, "DENIED")
	}
	if decoded.Operation != "delete" {
		t.Errorf("operation: got %q, want %q", decoded.Operation, "delete")
	}
	if len(decoded.Resources) != 2 || decoded.Resources[0] != "secret/cert-a" || decoded.Resources[1] != "secret/cert-b" {
		t.Errorf("resources: got %v", decoded.Resources)
	}
	if decoded.Namespace != "istio-system" {
		t.Errorf("namespace: got %q", decoded.Namespace)
	}
	if decoded.Cluster != "prod-cluster" {
		t.Errorf("cluster: got %q", decoded.Cluster)
	}
	if decoded.Confirmed {
		t.Errorf("confirmed: got true, want false")
	}
	if decoded.Command != "delete secret cert-a cert-b -n istio-system" {
		t.Errorf("command: got %q", decoded.Command)
	}
}

func TestFormatJSONKeys(t *testing.T) {
	e := Entry{Timestamp: "t", Status: "EXECUTED", Operation: "delete"}
	got, err := formatJSON(e)
	if err != nil {
		t.Fatalf("formatJSON() error: %v", err)
	}
	for _, key := range []string{
		`"timestamp"`, `"status"`, `"operation"`, `"resources"`,
		`"namespace"`, `"cluster"`, `"confirmed"`, `"command"`,
	} {
		if !strings.Contains(got, key) {
			t.Errorf("formatJSON() missing key %s, got: %s", key, got)
		}
	}
}

func TestLogJSONFormat(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	cfg := &config.Config{
		Audit: config.AuditConfig{
			Enabled: true,
			Path:    logPath,
			Format:  "json",
		},
	}

	logger := New(cfg)
	result := &checker.CheckResult{
		Operation: "delete",
		Resources: []string{"secret/cert-a", "secret/cert-b"},
		Namespace: "istio-system",
		Cluster:   "prod-cluster",
	}

	if err := logger.Log(result, []string{"delete", "secret", "cert-a", "cert-b", "-n", "istio-system"}, true, true); err != nil {
		t.Fatalf("Log() returned error: %v", err)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	line := strings.TrimSpace(string(content))
	var e Entry
	if err := json.Unmarshal([]byte(line), &e); err != nil {
		t.Fatalf("log line is not valid JSON: %v\n%s", err, line)
	}

	if e.Status != "EXECUTED" {
		t.Errorf("status: got %q, want EXECUTED", e.Status)
	}
	if e.Operation != "delete" {
		t.Errorf("operation: got %q, want delete", e.Operation)
	}
	if len(e.Resources) != 2 {
		t.Errorf("resources: got %v, want 2 entries", e.Resources)
	}
	if e.Namespace != "istio-system" {
		t.Errorf("namespace: got %q, want istio-system", e.Namespace)
	}
	if !e.Confirmed {
		t.Errorf("confirmed: got false, want true")
	}
}

func TestLogResourcesJSONFormat(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	cfg := &config.Config{
		Audit: config.AuditConfig{
			Enabled: true,
			Path:    logPath,
			Format:  "json",
		},
	}

	logger := New(cfg)
	result := &checker.ResourceCheckResult{
		Operation: "apply",
		Cluster:   "prod-cluster",
		Resources: []manifest.Resource{
			{Kind: "Deployment", Name: "nginx", Namespace: "production"},
		},
	}

	if err := logger.LogResources(result, []string{"apply", "-f", "deploy.yaml"}, true, true); err != nil {
		t.Fatalf("LogResources() returned error: %v", err)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	line := strings.TrimSpace(string(content))
	var e Entry
	if err := json.Unmarshal([]byte(line), &e); err != nil {
		t.Fatalf("log line is not valid JSON: %v\n%s", err, line)
	}

	if e.Namespace != "" {
		t.Errorf("namespace: got %q, want empty for file-based path", e.Namespace)
	}
	if len(e.Resources) != 1 || e.Resources[0] != "Deployment/nginx@production" {
		t.Errorf("resources: got %v, want [Deployment/nginx@production]", e.Resources)
	}
}

func TestLogResourcesTextUnified(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	cfg := &config.Config{
		Audit: config.AuditConfig{
			Enabled: true,
			Path:    logPath,
			Format:  "text",
		},
	}

	logger := New(cfg)
	result := &checker.ResourceCheckResult{
		Operation: "apply",
		Cluster:   "prod-cluster",
		Resources: []manifest.Resource{
			{Kind: "Deployment", Name: "nginx", Namespace: "production"},
		},
	}

	if err := logger.LogResources(result, []string{"apply", "-f", "deploy.yaml"}, true, true); err != nil {
		t.Fatalf("LogResources() returned error: %v", err)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "resources=[Deployment/nginx@production] namespace= cluster=prod-cluster") {
		t.Errorf("file-based text line not in unified layout, got:\n%s", string(content))
	}
}

func TestLogUnknownFormatFallsBackToText(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	cfg := &config.Config{
		Audit: config.AuditConfig{
			Enabled: true,
			Path:    logPath,
			Format:  "yaml", // unrecognized → must fall back to text
		},
	}

	logger := New(cfg)
	result := &checker.CheckResult{
		Operation: "delete",
		Resources: []string{"pod/nginx"},
		Namespace: "default",
		Cluster:   "test-cluster",
	}

	if err := logger.Log(result, []string{"delete", "pod", "nginx"}, true, true); err != nil {
		t.Fatalf("Log() returned error: %v", err)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	line := strings.TrimSpace(string(content))
	if strings.HasPrefix(line, "{") {
		t.Errorf("unknown format should produce text, got JSON: %s", line)
	}
	if !strings.Contains(line, "operation=delete resources=[pod/nginx]") {
		t.Errorf("unknown format did not produce text layout, got: %s", line)
	}
}
