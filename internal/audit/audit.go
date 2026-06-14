package audit

import (
	"encoding/json"
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

// Entry is a single audit record, rendered as text or JSON.
type Entry struct {
	Timestamp string   `json:"timestamp"`
	Status    string   `json:"status"` // EXECUTED | DENIED
	Operation string   `json:"operation"`
	Resources []string `json:"resources"`
	Namespace string   `json:"namespace"` // empty for file-based commands
	Cluster   string   `json:"cluster"`
	Confirmed bool     `json:"confirmed"`
	Command   string   `json:"command"`
}

// formatText renders an entry as the key=value audit line (no trailing newline).
// Uses literal quotes around the command so embedded quotes are preserved as-is.
func formatText(e Entry) string {
	return fmt.Sprintf("[%s] %s | operation=%s resources=[%s] namespace=%s cluster=%s confirmed=%t command=\"%s\"",
		e.Timestamp,
		e.Status,
		e.Operation,
		strings.Join(e.Resources, ","),
		e.Namespace,
		e.Cluster,
		e.Confirmed,
		e.Command,
	)
}

// formatJSON renders an entry as a single-line JSON object (no trailing newline).
func formatJSON(e Entry) (string, error) {
	b, err := json.Marshal(e)
	if err != nil {
		return "", fmt.Errorf("failed to marshal audit entry: %w", err)
	}
	return string(b), nil
}

// writeEntry persists one audit entry if auditing is enabled, choosing the
// output format from config (only "json" selects JSON; anything else is text).
func (l *Logger) writeEntry(e Entry) error {
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

	var line string
	if l.config.Audit.Format == "json" {
		line, err = formatJSON(e)
		if err != nil {
			return err
		}
	} else {
		line = formatText(e)
	}

	if _, err := file.WriteString(line + "\n"); err != nil {
		return fmt.Errorf("failed to write audit log: %w", err)
	}

	return nil
}

// Log writes an audit entry for CLI commands if auditing is enabled
func (l *Logger) Log(result *checker.CheckResult, args []string, confirmed bool, executed bool) error {
	status := "DENIED"
	if executed {
		status = "EXECUTED"
	}

	entry := Entry{
		Timestamp: time.Now().Format(time.RFC3339),
		Status:    status,
		Operation: result.Operation,
		Resources: result.Resources,
		Namespace: result.Namespace,
		Cluster:   result.Cluster,
		Confirmed: confirmed,
		Command:   strings.Join(args, " "),
	}

	return l.writeEntry(entry)
}

// LogResources writes an audit entry for file-based commands if auditing is enabled
func (l *Logger) LogResources(result *checker.ResourceCheckResult, args []string, confirmed bool, executed bool) error {
	status := "DENIED"
	if executed {
		status = "EXECUTED"
	}

	// Build resource list (namespace is baked into each entry)
	var resourceList []string
	for _, r := range result.Resources {
		ns := r.Namespace
		if ns == "" {
			ns = "default"
		}
		resourceList = append(resourceList, fmt.Sprintf("%s/%s@%s", r.Kind, r.Name, ns))
	}

	entry := Entry{
		Timestamp: time.Now().Format(time.RFC3339),
		Status:    status,
		Operation: result.Operation,
		Resources: resourceList,
		Namespace: "", // file-based: namespace is per-resource in the strings
		Cluster:   result.Cluster,
		Confirmed: confirmed,
		Command:   strings.Join(args, " "),
	}

	return l.writeEntry(entry)
}
