package checker

import (
	"github.com/zufardhiyaulhaq/safekubectl/internal/config"
	"github.com/zufardhiyaulhaq/safekubectl/internal/parser"
)

// CheckResult contains the result of a danger check
type CheckResult struct {
	IsDangerous          bool
	RequiresConfirmation bool
	IsNodeScoped         bool
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
		Operation:    cmd.Operation,
		Resource:     cmd.GetResourceDisplay(),
		Namespace:    namespace,
		Cluster:      cluster,
		IsNodeScoped: isNodeScoped,
		Reasons:      []string{},
	}

	// Only check if operation is dangerous first
	if !c.config.IsDangerousOperation(cmd.Operation) {
		// Safe operations pass through without warning
		return result
	}

	result.IsDangerous = true
	result.Reasons = append(result.Reasons, "dangerous operation: "+cmd.Operation)

	// Add additional context if in protected namespace/cluster
	if c.config.IsProtectedNamespace(namespace) {
		result.Reasons = append(result.Reasons, "protected namespace: "+namespace)
	}
	if c.config.IsProtectedCluster(cluster) {
		result.Reasons = append(result.Reasons, "protected cluster: "+cluster)
	}

	// Determine if confirmation is required
	result.RequiresConfirmation = c.config.RequiresConfirmation(namespace, cluster)

	return result
}
