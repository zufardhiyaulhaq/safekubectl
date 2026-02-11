package checker

import (
	"github.com/zufardhiyaulhaq/safekubectl/internal/config"
	"github.com/zufardhiyaulhaq/safekubectl/internal/manifest"
	"github.com/zufardhiyaulhaq/safekubectl/internal/parser"
)

// CheckResult contains the result of a danger check
type CheckResult struct {
	IsDangerous          bool
	RequiresConfirmation bool
	IsNodeScoped         bool
	IsAllNamespaces      bool
	IsDryRun             bool
	Operation            string
	Resource             string
	Namespace            string
	Cluster              string
	Reasons              []string
}

// Checker checks if kubectl commands are dangerous
type Checker struct {
	config *config.Config
}

// New creates a new Checker
func New(cfg *config.Config) *Checker {
	return &Checker{
		config: cfg,
	}
}

// Check analyzes a kubectl command and returns check result
func (c *Checker) Check(cmd *parser.KubectlCommand, cluster string) *CheckResult {
	namespace := cmd.GetNamespaceDisplay()
	isNodeScoped := cmd.IsNodeScoped()

	result := &CheckResult{
		Operation:       cmd.Operation,
		Resource:        cmd.GetResourceDisplay(),
		Namespace:       namespace,
		Cluster:         cluster,
		IsNodeScoped:    isNodeScoped,
		IsAllNamespaces: cmd.AllNamespaces,
		IsDryRun:        cmd.DryRun,
		Reasons:         []string{},
	}

	// Dry-run commands are safe - they don't actually execute
	if cmd.DryRun {
		return result
	}

	// Only check if operation is dangerous first
	if !c.config.IsDangerousOperation(cmd.Operation) {
		// Safe operations pass through without warning
		return result
	}

	result.IsDangerous = true
	result.Reasons = append(result.Reasons, "dangerous operation: "+cmd.Operation)

	// All-namespaces is especially dangerous
	if cmd.AllNamespaces {
		result.Reasons = append(result.Reasons, "AFFECTS ALL NAMESPACES (-A/--all-namespaces)")
		result.RequiresConfirmation = true // Always require confirmation for all-namespaces
	}

	// Add additional context if in protected namespace/cluster (only if not all-namespaces)
	if !cmd.AllNamespaces && !isNodeScoped && c.config.IsProtectedNamespace(namespace) {
		result.Reasons = append(result.Reasons, "protected namespace: "+namespace)
	}
	if c.config.IsProtectedCluster(cluster) {
		result.Reasons = append(result.Reasons, "protected cluster: "+cluster)
	}

	// Determine if confirmation is required
	if !result.RequiresConfirmation {
		result.RequiresConfirmation = c.config.RequiresConfirmation(namespace, cluster)
	}

	return result
}

// ResourceCheckResult contains check result for file-based commands
type ResourceCheckResult struct {
	IsDangerous          bool
	RequiresConfirmation bool
	Operation            string
	Cluster              string
	Resources            []manifest.Resource
	Reasons              []string
}

// CheckResources analyzes multiple resources from manifest files
func (c *Checker) CheckResources(operation string, resources []manifest.Resource, cluster string) *ResourceCheckResult {
	result := &ResourceCheckResult{
		Operation: operation,
		Cluster:   cluster,
		Resources: resources,
		Reasons:   []string{},
	}

	// Check if operation is dangerous
	if !c.config.IsDangerousOperation(operation) {
		return result
	}

	result.IsDangerous = true
	result.Reasons = append(result.Reasons, "dangerous operation: "+operation)

	// Check each resource's namespace
	protectedNamespaces := make(map[string]bool)
	for _, r := range resources {
		ns := r.Namespace
		if ns == "" {
			ns = "default"
		}
		if c.config.IsProtectedNamespace(ns) {
			protectedNamespaces[ns] = true
		}
	}

	for ns := range protectedNamespaces {
		result.Reasons = append(result.Reasons, "protected namespace: "+ns)
	}

	// Check protected cluster
	if c.config.IsProtectedCluster(cluster) {
		result.Reasons = append(result.Reasons, "protected cluster: "+cluster)
	}

	// Determine if confirmation required
	result.RequiresConfirmation = c.config.Mode == config.ModeConfirm
	if !result.RequiresConfirmation {
		// In warn-only mode, still require confirmation for protected resources
		if len(protectedNamespaces) > 0 || c.config.IsProtectedCluster(cluster) {
			result.RequiresConfirmation = true
		}
	}

	return result
}
