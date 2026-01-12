package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/zufardhiyaulhaq/safekubectl/internal/audit"
	"github.com/zufardhiyaulhaq/safekubectl/internal/checker"
	"github.com/zufardhiyaulhaq/safekubectl/internal/config"
	"github.com/zufardhiyaulhaq/safekubectl/internal/parser"
	"github.com/zufardhiyaulhaq/safekubectl/internal/prompt"
)

func main() {
	runner := &Runner{
		stdin:          os.Stdin,
		stdout:         os.Stdout,
		stderr:         os.Stderr,
		getCluster:     getCurrentCluster,
		executeKubectl: executeKubectl,
		loadConfig:     config.Load,
	}

	if err := runner.Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "safekubectl: %s\n", err)
		os.Exit(1)
	}
}

// Runner encapsulates the main execution logic
type Runner struct {
	stdin          io.Reader
	stdout         io.Writer
	stderr         io.Writer
	getCluster     func() string
	executeKubectl func(args []string) error
	loadConfig     func() (*config.Config, error)
}

// Run executes the main logic
func (r *Runner) Run(args []string) error {
	// If no args, just pass through to kubectl
	if len(args) == 0 {
		return r.executeKubectl(args)
	}

	// Load configuration
	cfg, err := r.loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Parse kubectl command
	cmd := parser.Parse(args)

	// Get cluster context - use parsed --context flag if provided, otherwise get current context
	cluster := cmd.Context
	if cluster == "" {
		cluster = r.getCluster()
	}

	// Check if command is dangerous
	chk := checker.New(cfg)
	result := chk.Check(cmd, cluster)

	// Initialize audit logger
	auditLogger := audit.New(cfg)

	// If not dangerous, execute directly
	if !result.IsDangerous {
		return r.executeKubectl(args)
	}

	// Display warning
	prompt.DisplayWarningTo(r.stdout, result)

	// Handle based on confirmation requirement
	confirmed := false
	if result.RequiresConfirmation {
		confirmed = prompt.AskConfirmationFrom(r.stdin, r.stdout)
		if !confirmed {
			prompt.DisplayAbortedTo(r.stdout)
			// Log denied operation
			if err := auditLogger.Log(result, args, false, false); err != nil {
				fmt.Fprintf(r.stderr, "warning: failed to write audit log: %s\n", err)
			}
			return nil
		}
	} else {
		// Warn-only mode (unless protected)
		prompt.DisplayProceedingTo(r.stdout)
		confirmed = true
	}

	// Log the operation
	if err := auditLogger.Log(result, args, confirmed, true); err != nil {
		fmt.Fprintf(r.stderr, "warning: failed to write audit log: %s\n", err)
	}

	// Execute kubectl
	return r.executeKubectl(args)
}

// getCurrentCluster gets the current kubernetes context/cluster name
func getCurrentCluster() string {
	cmd := exec.Command("kubectl", "config", "current-context")
	output, err := cmd.Output()
	if err != nil {
		return "<unknown>"
	}
	return strings.TrimSpace(string(output))
}

// executeKubectl runs kubectl with the given arguments
func executeKubectl(args []string) error {
	kubectl, err := exec.LookPath("kubectl")
	if err != nil {
		return fmt.Errorf("kubectl not found in PATH: %w", err)
	}

	cmd := exec.Command(kubectl, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return err
	}

	return nil
}
