package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zufardhiyaulhaq/safekubectl/internal/checker"
	"github.com/zufardhiyaulhaq/safekubectl/internal/config"
)

// Logger handles audit logging
type Logger struct {
	config *config.Config
}

// New creates a new audit Logger
func New(cfg *config.Config) *Logger {
	return &Logger{
		config: cfg,
	}
}

// Log writes an audit entry if auditing is enabled
func (l *Logger) Log(result *checker.CheckResult, args []string, confirmed bool, executed bool) error {
	if !l.config.Audit.Enabled {
		return nil
	}

	// Ensure directory exists
	dir := filepath.Dir(l.config.Audit.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create audit directory: %w", err)
	}

	// Open file in append mode
	file, err := os.OpenFile(l.config.Audit.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open audit log: %w", err)
	}
	defer file.Close()

	// Format audit entry
	timestamp := time.Now().Format(time.RFC3339)
	status := "DENIED"
	if executed {
		status = "EXECUTED"
	}

	entry := fmt.Sprintf("[%s] %s | operation=%s resource=%s namespace=%s cluster=%s confirmed=%t command=\"%s\"\n",
		timestamp,
		status,
		result.Operation,
		result.Resource,
		result.Namespace,
		result.Cluster,
		confirmed,
		strings.Join(args, " "),
	)

	if _, err := file.WriteString(entry); err != nil {
		return fmt.Errorf("failed to write audit log: %w", err)
	}

	return nil
}

// LogResources writes an audit entry for file-based commands if auditing is enabled
func (l *Logger) LogResources(result *checker.ResourceCheckResult, args []string, confirmed bool, executed bool) error {
	if !l.config.Audit.Enabled {
		return nil
	}

	// Ensure directory exists
	dir := filepath.Dir(l.config.Audit.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create audit directory: %w", err)
	}

	// Open file in append mode
	file, err := os.OpenFile(l.config.Audit.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open audit log: %w", err)
	}
	defer file.Close()

	// Format audit entry
	timestamp := time.Now().Format(time.RFC3339)
	status := "DENIED"
	if executed {
		status = "EXECUTED"
	}

	// Build resource list
	var resourceList []string
	for _, r := range result.Resources {
		ns := r.Namespace
		if ns == "" {
			ns = "default"
		}
		resourceList = append(resourceList, fmt.Sprintf("%s/%s@%s", r.Kind, r.Name, ns))
	}

	entry := fmt.Sprintf("[%s] %s | operation=%s cluster=%s resources=[%s] confirmed=%t command=\"%s\"\n",
		timestamp,
		status,
		result.Operation,
		result.Cluster,
		strings.Join(resourceList, ","),
		confirmed,
		strings.Join(args, " "),
	)

	if _, err := file.WriteString(entry); err != nil {
		return fmt.Errorf("failed to write audit log: %w", err)
	}

	return nil
}
