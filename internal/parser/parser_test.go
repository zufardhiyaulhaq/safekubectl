package parser

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected *KubectlCommand
	}{
		{
			name: "simple delete pod",
			args: []string{"delete", "pod", "nginx"},
			expected: &KubectlCommand{
				Operation: "delete",
				Resource:  "pod",
				Name:      "nginx",
				Namespace: "",
				Context:   "",
				Args:      []string{"delete", "pod", "nginx"},
			},
		},
		{
			name: "delete with resource/name format",
			args: []string{"delete", "pod/nginx"},
			expected: &KubectlCommand{
				Operation: "delete",
				Resource:  "pod",
				Name:      "nginx",
				Namespace: "",
				Args:      []string{"delete", "pod/nginx"},
			},
		},
		{
			name: "delete with namespace flag -n",
			args: []string{"delete", "pod", "nginx", "-n", "production"},
			expected: &KubectlCommand{
				Operation: "delete",
				Resource:  "pod",
				Name:      "nginx",
				Namespace: "production",
				Args:      []string{"delete", "pod", "nginx", "-n", "production"},
			},
		},
		{
			name: "delete with namespace flag --namespace",
			args: []string{"delete", "pod", "nginx", "--namespace", "production"},
			expected: &KubectlCommand{
				Operation: "delete",
				Resource:  "pod",
				Name:      "nginx",
				Namespace: "production",
				Args:      []string{"delete", "pod", "nginx", "--namespace", "production"},
			},
		},
		{
			name: "delete with namespace flag -n=value",
			args: []string{"delete", "pod", "nginx", "-n=production"},
			expected: &KubectlCommand{
				Operation: "delete",
				Resource:  "pod",
				Name:      "nginx",
				Namespace: "production",
				Args:      []string{"delete", "pod", "nginx", "-n=production"},
			},
		},
		{
			name: "delete with namespace flag --namespace=value",
			args: []string{"delete", "pod", "nginx", "--namespace=production"},
			expected: &KubectlCommand{
				Operation: "delete",
				Resource:  "pod",
				Name:      "nginx",
				Namespace: "production",
				Args:      []string{"delete", "pod", "nginx", "--namespace=production"},
			},
		},
		{
			name: "namespace flag before operation",
			args: []string{"-n", "production", "delete", "pod", "nginx"},
			expected: &KubectlCommand{
				Operation: "delete",
				Resource:  "pod",
				Name:      "nginx",
				Namespace: "production",
				Args:      []string{"-n", "production", "delete", "pod", "nginx"},
			},
		},
		{
			name: "get pods (safe operation)",
			args: []string{"get", "pods"},
			expected: &KubectlCommand{
				Operation: "get",
				Resource:  "pods",
				Name:      "",
				Namespace: "",
				Args:      []string{"get", "pods"},
			},
		},
		{
			name: "apply with file flag",
			args: []string{"apply", "-f", "deployment.yaml"},
			expected: &KubectlCommand{
				Operation: "apply",
				Resource:  "",
				Name:      "",
				Namespace: "",
				Args:      []string{"apply", "-f", "deployment.yaml"},
			},
		},
		{
			name: "apply with file and namespace",
			args: []string{"apply", "-f", "deployment.yaml", "-n", "staging"},
			expected: &KubectlCommand{
				Operation: "apply",
				Resource:  "",
				Name:      "",
				Namespace: "staging",
				Args:      []string{"apply", "-f", "deployment.yaml", "-n", "staging"},
			},
		},
		{
			name: "exec command",
			args: []string{"exec", "-it", "nginx", "--", "/bin/sh"},
			expected: &KubectlCommand{
				Operation: "exec",
				Resource:  "nginx",
				Name:      "/bin/sh",
				Namespace: "",
				Args:      []string{"exec", "-it", "nginx", "--", "/bin/sh"},
			},
		},
		{
			name: "rollout restart deployment",
			args: []string{"rollout", "restart", "deployment/nginx"},
			expected: &KubectlCommand{
				Operation: "rollout",
				Resource:  "restart",
				Name:      "deployment/nginx",
				Namespace: "",
				Args:      []string{"rollout", "restart", "deployment/nginx"},
			},
		},
		{
			name: "drain node",
			args: []string{"drain", "node-1", "--ignore-daemonsets"},
			expected: &KubectlCommand{
				Operation: "drain",
				Resource:  "node-1",
				Name:      "",
				Namespace: "",
				Args:      []string{"drain", "node-1", "--ignore-daemonsets"},
			},
		},
		{
			name: "cordon node",
			args: []string{"cordon", "node-1"},
			expected: &KubectlCommand{
				Operation: "cordon",
				Resource:  "node-1",
				Name:      "",
				Namespace: "",
				Args:      []string{"cordon", "node-1"},
			},
		},
		{
			name: "taint node",
			args: []string{"taint", "nodes", "node-1", "key=value:NoSchedule"},
			expected: &KubectlCommand{
				Operation: "taint",
				Resource:  "nodes",
				Name:      "node-1",
				Namespace: "",
				Args:      []string{"taint", "nodes", "node-1", "key=value:NoSchedule"},
			},
		},
		{
			name: "patch deployment",
			args: []string{"patch", "deployment", "nginx", "-p", `{"spec":{"replicas":3}}`},
			expected: &KubectlCommand{
				Operation: "patch",
				Resource:  "deployment",
				Name:      "nginx",
				Namespace: "",
				Args:      []string{"patch", "deployment", "nginx", "-p", `{"spec":{"replicas":3}}`},
			},
		},
		{
			name: "edit configmap",
			args: []string{"edit", "configmap", "my-config", "-n", "default"},
			expected: &KubectlCommand{
				Operation: "edit",
				Resource:  "configmap",
				Name:      "my-config",
				Namespace: "default",
				Args:      []string{"edit", "configmap", "my-config", "-n", "default"},
			},
		},
		{
			name: "empty args",
			args: []string{},
			expected: &KubectlCommand{
				Operation: "",
				Resource:  "",
				Name:      "",
				Namespace: "",
				Args:      []string{},
			},
		},
		{
			name: "with output flag",
			args: []string{"get", "pods", "-o", "yaml"},
			expected: &KubectlCommand{
				Operation: "get",
				Resource:  "pods",
				Name:      "",
				Namespace: "",
				Args:      []string{"get", "pods", "-o", "yaml"},
			},
		},
		{
			name: "with selector flag",
			args: []string{"delete", "pods", "-l", "app=nginx", "-n", "default"},
			expected: &KubectlCommand{
				Operation: "delete",
				Resource:  "pods",
				Name:      "",
				Namespace: "default",
				Args:      []string{"delete", "pods", "-l", "app=nginx", "-n", "default"},
			},
		},
		{
			name: "with context flag before operation",
			args: []string{"--context", "prod-cluster", "delete", "pod", "nginx"},
			expected: &KubectlCommand{
				Operation: "delete",
				Resource:  "pod",
				Name:      "nginx",
				Namespace: "",
				Context:   "prod-cluster",
				Args:      []string{"--context", "prod-cluster", "delete", "pod", "nginx"},
			},
		},
		{
			name: "with context flag after operation",
			args: []string{"delete", "pod", "nginx", "--context", "prod-cluster"},
			expected: &KubectlCommand{
				Operation: "delete",
				Resource:  "pod",
				Name:      "nginx",
				Namespace: "",
				Context:   "prod-cluster",
				Args:      []string{"delete", "pod", "nginx", "--context", "prod-cluster"},
			},
		},
		{
			name: "with context= flag",
			args: []string{"delete", "pod", "nginx", "--context=prod-cluster"},
			expected: &KubectlCommand{
				Operation: "delete",
				Resource:  "pod",
				Name:      "nginx",
				Namespace: "",
				Context:   "prod-cluster",
				Args:      []string{"delete", "pod", "nginx", "--context=prod-cluster"},
			},
		},
		{
			name: "with context and namespace",
			args: []string{"delete", "pod", "nginx", "-n", "production", "--context", "prod-cluster"},
			expected: &KubectlCommand{
				Operation: "delete",
				Resource:  "pod",
				Name:      "nginx",
				Namespace: "production",
				Context:   "prod-cluster",
				Args:      []string{"delete", "pod", "nginx", "-n", "production", "--context", "prod-cluster"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Parse(tt.args)

			if result.Operation != tt.expected.Operation {
				t.Errorf("Operation: got %q, expected %q", result.Operation, tt.expected.Operation)
			}

			if result.Resource != tt.expected.Resource {
				t.Errorf("Resource: got %q, expected %q", result.Resource, tt.expected.Resource)
			}

			if result.Name != tt.expected.Name {
				t.Errorf("Name: got %q, expected %q", result.Name, tt.expected.Name)
			}

			if result.Namespace != tt.expected.Namespace {
				t.Errorf("Namespace: got %q, expected %q", result.Namespace, tt.expected.Namespace)
			}

			if result.Context != tt.expected.Context {
				t.Errorf("Context: got %q, expected %q", result.Context, tt.expected.Context)
			}

			if !reflect.DeepEqual(result.Args, tt.expected.Args) {
				t.Errorf("Args: got %v, expected %v", result.Args, tt.expected.Args)
			}
		})
	}
}

func TestGetResourceDisplay(t *testing.T) {
	tests := []struct {
		name     string
		cmd      *KubectlCommand
		expected string
	}{
		{
			name:     "resource with name",
			cmd:      &KubectlCommand{Resource: "pod", Name: "nginx"},
			expected: "pod/nginx",
		},
		{
			name:     "resource without name",
			cmd:      &KubectlCommand{Resource: "pods", Name: ""},
			expected: "pods",
		},
		{
			name:     "empty resource",
			cmd:      &KubectlCommand{Resource: "", Name: ""},
			expected: "<unknown>",
		},
		{
			name:     "empty resource with name",
			cmd:      &KubectlCommand{Resource: "", Name: "nginx"},
			expected: "<unknown>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cmd.GetResourceDisplay()
			if result != tt.expected {
				t.Errorf("GetResourceDisplay() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestGetNamespaceDisplay(t *testing.T) {
	tests := []struct {
		name     string
		cmd      *KubectlCommand
		expected string
	}{
		{
			name:     "with namespace",
			cmd:      &KubectlCommand{Namespace: "production"},
			expected: "production",
		},
		{
			name:     "empty namespace",
			cmd:      &KubectlCommand{Namespace: ""},
			expected: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cmd.GetNamespaceDisplay()
			if result != tt.expected {
				t.Errorf("GetNamespaceDisplay() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestIsNodeScoped(t *testing.T) {
	tests := []struct {
		name     string
		cmd      *KubectlCommand
		expected bool
	}{
		{
			name:     "cordon is node-scoped",
			cmd:      &KubectlCommand{Operation: "cordon"},
			expected: true,
		},
		{
			name:     "uncordon is node-scoped",
			cmd:      &KubectlCommand{Operation: "uncordon"},
			expected: true,
		},
		{
			name:     "drain is node-scoped",
			cmd:      &KubectlCommand{Operation: "drain"},
			expected: true,
		},
		{
			name:     "taint is node-scoped",
			cmd:      &KubectlCommand{Operation: "taint"},
			expected: true,
		},
		{
			name:     "delete is not node-scoped",
			cmd:      &KubectlCommand{Operation: "delete"},
			expected: false,
		},
		{
			name:     "apply is not node-scoped",
			cmd:      &KubectlCommand{Operation: "apply"},
			expected: false,
		},
		{
			name:     "get is not node-scoped",
			cmd:      &KubectlCommand{Operation: "get"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cmd.IsNodeScoped()
			if result != tt.expected {
				t.Errorf("IsNodeScoped() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestNeedsValue(t *testing.T) {
	tests := []struct {
		flag     string
		expected bool
	}{
		{"-n", true},
		{"--namespace", true},
		{"-f", true},
		{"--filename", true},
		{"-l", true},
		{"--selector", true},
		{"-o", true},
		{"--output", true},
		{"--context", true},
		{"--cluster", true},
		{"-c", true},
		{"--container", true},
		{"-p", true},
		{"--patch", true},
		{"--type", true},
		{"--timeout", true},
		{"--grace-period", true},
		{"--all", false},
		{"--force", false},
		{"--ignore-daemonsets", false},
		{"-it", false},
		{"--dry-run", false},
		{"-n=value", true},
	}

	for _, tt := range tests {
		t.Run(tt.flag, func(t *testing.T) {
			result := needsValue(tt.flag)
			if result != tt.expected {
				t.Errorf("needsValue(%q) = %v, expected %v", tt.flag, result, tt.expected)
			}
		})
	}
}
