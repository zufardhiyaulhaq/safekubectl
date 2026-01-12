package main

import (
	"bytes"
	"errors"
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
