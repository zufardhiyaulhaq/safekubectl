package checker

import (
	"testing"

	"github.com/zufardhiyaulhaq/safekubectl/internal/config"
	"github.com/zufardhiyaulhaq/safekubectl/internal/manifest"
	"github.com/zufardhiyaulhaq/safekubectl/internal/parser"
)

func TestNew(t *testing.T) {
	cfg := config.DefaultConfig()
	chk := New(cfg)

	if chk == nil {
		t.Fatal("New() returned nil")
	}

	if chk.config != cfg {
		t.Error("New() did not set config correctly")
	}
}

func TestCheck(t *testing.T) {
	tests := []struct {
		name                 string
		config               *config.Config
		args                 []string
		cluster              string
		expectedDangerous    bool
		expectedConfirmation bool
		expectedReasonsCount int
	}{
		{
			name: "safe operation - get pods",
			config: &config.Config{
				Mode:                config.ModeConfirm,
				DangerousOperations: []string{"delete", "apply"},
				ProtectedNamespaces: []string{"kube-system"},
				ProtectedClusters:   []string{},
			},
			args:                 []string{"get", "pods"},
			cluster:              "dev-cluster",
			expectedDangerous:    false,
			expectedConfirmation: false,
			expectedReasonsCount: 0,
		},
		{
			name: "dangerous operation - delete pod",
			config: &config.Config{
				Mode:                config.ModeConfirm,
				DangerousOperations: []string{"delete", "apply"},
				ProtectedNamespaces: []string{},
				ProtectedClusters:   []string{},
			},
			args:                 []string{"delete", "pod", "nginx"},
			cluster:              "dev-cluster",
			expectedDangerous:    true,
			expectedConfirmation: true,
			expectedReasonsCount: 1,
		},
		{
			name: "dangerous operation in protected namespace",
			config: &config.Config{
				Mode:                config.ModeConfirm,
				DangerousOperations: []string{"delete"},
				ProtectedNamespaces: []string{"kube-system"},
				ProtectedClusters:   []string{},
			},
			args:                 []string{"delete", "pod", "nginx", "-n", "kube-system"},
			cluster:              "dev-cluster",
			expectedDangerous:    true,
			expectedConfirmation: true,
			expectedReasonsCount: 2, // dangerous op + protected namespace
		},
		{
			name: "dangerous operation in protected cluster",
			config: &config.Config{
				Mode:                config.ModeConfirm,
				DangerousOperations: []string{"delete"},
				ProtectedNamespaces: []string{},
				ProtectedClusters:   []string{"prod-cluster"},
			},
			args:                 []string{"delete", "pod", "nginx"},
			cluster:              "prod-cluster",
			expectedDangerous:    true,
			expectedConfirmation: true,
			expectedReasonsCount: 2, // dangerous op + protected cluster
		},
		{
			name: "dangerous operation in both protected namespace and cluster",
			config: &config.Config{
				Mode:                config.ModeConfirm,
				DangerousOperations: []string{"delete"},
				ProtectedNamespaces: []string{"production"},
				ProtectedClusters:   []string{"prod-cluster"},
			},
			args:                 []string{"delete", "pod", "nginx", "-n", "production"},
			cluster:              "prod-cluster",
			expectedDangerous:    true,
			expectedConfirmation: true,
			expectedReasonsCount: 3, // dangerous op + protected namespace + protected cluster
		},
		{
			name: "warn-only mode - no confirmation required for non-protected",
			config: &config.Config{
				Mode:                config.ModeWarnOnly,
				DangerousOperations: []string{"delete"},
				ProtectedNamespaces: []string{"production"},
				ProtectedClusters:   []string{},
			},
			args:                 []string{"delete", "pod", "nginx", "-n", "staging"},
			cluster:              "dev-cluster",
			expectedDangerous:    true,
			expectedConfirmation: false,
			expectedReasonsCount: 1,
		},
		{
			name: "warn-only mode - confirmation required for protected namespace",
			config: &config.Config{
				Mode:                config.ModeWarnOnly,
				DangerousOperations: []string{"delete"},
				ProtectedNamespaces: []string{"production"},
				ProtectedClusters:   []string{},
			},
			args:                 []string{"delete", "pod", "nginx", "-n", "production"},
			cluster:              "dev-cluster",
			expectedDangerous:    true,
			expectedConfirmation: true,
			expectedReasonsCount: 2,
		},
		{
			name: "warn-only mode - confirmation required for protected cluster",
			config: &config.Config{
				Mode:                config.ModeWarnOnly,
				DangerousOperations: []string{"delete"},
				ProtectedNamespaces: []string{},
				ProtectedClusters:   []string{"prod-cluster"},
			},
			args:                 []string{"delete", "pod", "nginx"},
			cluster:              "prod-cluster",
			expectedDangerous:    true,
			expectedConfirmation: true,
			expectedReasonsCount: 2,
		},
		{
			name: "safe operation in protected namespace - no warning",
			config: &config.Config{
				Mode:                config.ModeConfirm,
				DangerousOperations: []string{"delete", "apply"},
				ProtectedNamespaces: []string{"kube-system"},
				ProtectedClusters:   []string{},
			},
			args:                 []string{"get", "pods", "-n", "kube-system"},
			cluster:              "dev-cluster",
			expectedDangerous:    false,
			expectedConfirmation: false,
			expectedReasonsCount: 0,
		},
		{
			name: "apply operation",
			config: &config.Config{
				Mode:                config.ModeConfirm,
				DangerousOperations: []string{"apply"},
				ProtectedNamespaces: []string{},
				ProtectedClusters:   []string{},
			},
			args:                 []string{"apply", "-f", "deployment.yaml"},
			cluster:              "dev-cluster",
			expectedDangerous:    true,
			expectedConfirmation: true,
			expectedReasonsCount: 1,
		},
		{
			name: "exec operation",
			config: &config.Config{
				Mode:                config.ModeConfirm,
				DangerousOperations: []string{"exec"},
				ProtectedNamespaces: []string{},
				ProtectedClusters:   []string{},
			},
			args:                 []string{"exec", "-it", "nginx", "--", "/bin/sh"},
			cluster:              "dev-cluster",
			expectedDangerous:    true,
			expectedConfirmation: true,
			expectedReasonsCount: 1,
		},
		{
			name: "rollout operation",
			config: &config.Config{
				Mode:                config.ModeConfirm,
				DangerousOperations: []string{"rollout"},
				ProtectedNamespaces: []string{},
				ProtectedClusters:   []string{},
			},
			args:                 []string{"rollout", "restart", "deployment/nginx"},
			cluster:              "dev-cluster",
			expectedDangerous:    true,
			expectedConfirmation: true,
			expectedReasonsCount: 1,
		},
		{
			name: "drain operation",
			config: &config.Config{
				Mode:                config.ModeConfirm,
				DangerousOperations: []string{"drain"},
				ProtectedNamespaces: []string{},
				ProtectedClusters:   []string{},
			},
			args:                 []string{"drain", "node-1"},
			cluster:              "dev-cluster",
			expectedDangerous:    true,
			expectedConfirmation: true,
			expectedReasonsCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chk := New(tt.config)
			cmd := parser.Parse(tt.args)
			result := chk.Check(cmd, tt.cluster)

			if result.IsDangerous != tt.expectedDangerous {
				t.Errorf("IsDangerous: got %v, expected %v", result.IsDangerous, tt.expectedDangerous)
			}

			if result.RequiresConfirmation != tt.expectedConfirmation {
				t.Errorf("RequiresConfirmation: got %v, expected %v", result.RequiresConfirmation, tt.expectedConfirmation)
			}

			if len(result.Reasons) != tt.expectedReasonsCount {
				t.Errorf("Reasons count: got %d, expected %d (reasons: %v)", len(result.Reasons), tt.expectedReasonsCount, result.Reasons)
			}

			if result.Cluster != tt.cluster {
				t.Errorf("Cluster: got %q, expected %q", result.Cluster, tt.cluster)
			}
		})
	}
}

func TestCheckResultFields(t *testing.T) {
	cfg := &config.Config{
		Mode:                config.ModeConfirm,
		DangerousOperations: []string{"delete"},
		ProtectedNamespaces: []string{},
		ProtectedClusters:   []string{},
	}

	chk := New(cfg)
	cmd := parser.Parse([]string{"delete", "pod", "nginx", "-n", "production"})
	result := chk.Check(cmd, "prod-cluster")

	if result.Operation != "delete" {
		t.Errorf("Operation: got %q, expected %q", result.Operation, "delete")
	}

	if result.Resource != "pod/nginx" {
		t.Errorf("Resource: got %q, expected %q", result.Resource, "pod/nginx")
	}

	if result.Namespace != "production" {
		t.Errorf("Namespace: got %q, expected %q", result.Namespace, "production")
	}

	if result.Cluster != "prod-cluster" {
		t.Errorf("Cluster: got %q, expected %q", result.Cluster, "prod-cluster")
	}
}

func TestCheckWithDefaultNamespace(t *testing.T) {
	cfg := &config.Config{
		Mode:                config.ModeConfirm,
		DangerousOperations: []string{"delete"},
		ProtectedNamespaces: []string{"default"},
		ProtectedClusters:   []string{},
	}

	chk := New(cfg)
	// No namespace specified, should default to "default"
	cmd := parser.Parse([]string{"delete", "pod", "nginx"})
	result := chk.Check(cmd, "dev-cluster")

	if result.Namespace != "default" {
		t.Errorf("Namespace: got %q, expected %q", result.Namespace, "default")
	}

	// Should be marked as protected since default namespace is in the protected list
	if len(result.Reasons) != 2 {
		t.Errorf("Expected 2 reasons (dangerous op + protected namespace), got %d: %v", len(result.Reasons), result.Reasons)
	}
}

func TestCheckEmptyArgs(t *testing.T) {
	cfg := config.DefaultConfig()
	chk := New(cfg)
	cmd := parser.Parse([]string{})
	result := chk.Check(cmd, "dev-cluster")

	if result.IsDangerous {
		t.Error("Empty args should not be dangerous")
	}

	if result.Operation != "" {
		t.Errorf("Operation should be empty, got %q", result.Operation)
	}
}

func TestCheckResources(t *testing.T) {
	cfg := &config.Config{
		Mode:                config.ModeConfirm,
		DangerousOperations: []string{"apply", "delete"},
		ProtectedNamespaces: []string{"istio-system", "kube-system"},
		ProtectedClusters:   []string{"prod-cluster"},
	}

	chk := New(cfg)

	resources := []manifest.Resource{
		{Kind: "Deployment", Name: "nginx", Namespace: "istio-system", Source: "deploy.yaml"},
		{Kind: "Service", Name: "nginx-svc", Namespace: "default", Source: "deploy.yaml"},
	}

	result := chk.CheckResources("apply", resources, "dev-cluster")

	if !result.IsDangerous {
		t.Error("Expected IsDangerous=true for apply operation")
	}

	if len(result.Reasons) < 2 {
		t.Errorf("Expected at least 2 reasons, got %d: %v", len(result.Reasons), result.Reasons)
	}

	if !result.RequiresConfirmation {
		t.Error("Expected RequiresConfirmation=true")
	}
}

func TestCheckResourcesProtectedCluster(t *testing.T) {
	cfg := &config.Config{
		Mode:                config.ModeConfirm,
		DangerousOperations: []string{"apply"},
		ProtectedNamespaces: []string{},
		ProtectedClusters:   []string{"prod-cluster"},
	}

	chk := New(cfg)

	resources := []manifest.Resource{
		{Kind: "Deployment", Name: "nginx", Namespace: "default", Source: "deploy.yaml"},
	}

	result := chk.CheckResources("apply", resources, "prod-cluster")

	if !result.RequiresConfirmation {
		t.Error("Expected RequiresConfirmation=true for protected cluster")
	}
}

func TestCheckResourcesSafeOperation(t *testing.T) {
	cfg := &config.Config{
		Mode:                config.ModeConfirm,
		DangerousOperations: []string{"delete"},
		ProtectedNamespaces: []string{"kube-system"},
		ProtectedClusters:   []string{},
	}

	chk := New(cfg)

	resources := []manifest.Resource{
		{Kind: "Deployment", Name: "nginx", Namespace: "kube-system", Source: "deploy.yaml"},
	}

	result := chk.CheckResources("get", resources, "dev-cluster")

	if result.IsDangerous {
		t.Error("Expected IsDangerous=false for get operation")
	}
}
