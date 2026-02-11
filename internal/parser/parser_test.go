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
				Name:      "", // Args after -- are command args, not kubectl args
				Namespace: "",
				Args:      []string{"exec", "-it", "nginx", "--", "/bin/sh"},
			},
		},
		{
			name: "rollout restart deployment",
			args: []string{"rollout", "restart", "deployment/nginx"},
			expected: &KubectlCommand{
				Operation: "rollout",
				Resource:  "deployment",
				Name:      "nginx",
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

func TestLogsWithFollowFlag(t *testing.T) {
	// Bug: -f in "logs -f" means "follow", NOT file input
	// Should NOT treat --tail as a file path
	args := []string{"logs", "nginx-pod", "-f", "--tail", "100"}
	result := Parse(args)

	if result.Operation != "logs" {
		t.Errorf("Operation = %q, expected %q", result.Operation, "logs")
	}
	if result.Resource != "nginx-pod" {
		t.Errorf("Resource = %q, expected %q", result.Resource, "nginx-pod")
	}
	// Should NOT have any file inputs
	if len(result.FileInputs) != 0 {
		t.Errorf("FileInputs = %v, expected empty (logs -f means follow, not file)", result.FileInputs)
	}
}

func TestRolloutRestartParsing(t *testing.T) {
	// Bug: rollout restart deploy/nginx should show Resource=deployment, Name=nginx
	// Not Resource=restart, Name=deploy/nginx
	tests := []struct {
		name             string
		args             []string
		expectedResource string
		expectedName     string
	}{
		{
			name:             "rollout restart with resource/name",
			args:             []string{"rollout", "restart", "deploy/nginx"},
			expectedResource: "deploy",
			expectedName:     "nginx",
		},
		{
			name:             "rollout restart with separate resource and name",
			args:             []string{"rollout", "restart", "deployment", "nginx"},
			expectedResource: "deployment",
			expectedName:     "nginx",
		},
		{
			name:             "rollout status",
			args:             []string{"rollout", "status", "deployment/nginx"},
			expectedResource: "deployment",
			expectedName:     "nginx",
		},
		{
			name:             "rollout undo",
			args:             []string{"rollout", "undo", "deployment", "nginx"},
			expectedResource: "deployment",
			expectedName:     "nginx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Parse(tt.args)

			if result.Resource != tt.expectedResource {
				t.Errorf("Resource = %q, expected %q", result.Resource, tt.expectedResource)
			}
			if result.Name != tt.expectedName {
				t.Errorf("Name = %q, expected %q", result.Name, tt.expectedName)
			}
		})
	}
}

func TestAllNamespacesFlag(t *testing.T) {
	tests := []struct {
		name              string
		args              []string
		expectedAllNS     bool
	}{
		{"long flag", []string{"delete", "pods", "--all", "--all-namespaces"}, true},
		{"short flag", []string{"delete", "pods", "--all", "-A"}, true},
		{"flag before operation", []string{"-A", "get", "pods"}, true},
		{"no flag", []string{"delete", "pods"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Parse(tt.args)

			if result.AllNamespaces != tt.expectedAllNS {
				t.Errorf("AllNamespaces = %v, expected %v", result.AllNamespaces, tt.expectedAllNS)
			}
		})
	}
}

func TestDryRunFlag(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectedDryRun bool
	}{
		{"dry-run=client", []string{"delete", "pod", "nginx", "--dry-run=client"}, true},
		{"dry-run=server", []string{"delete", "pod", "nginx", "--dry-run=server"}, true},
		{"dry-run without value", []string{"delete", "pod", "nginx", "--dry-run"}, true},
		{"no dry-run", []string{"delete", "pod", "nginx"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Parse(tt.args)

			if result.DryRun != tt.expectedDryRun {
				t.Errorf("DryRun = %v, expected %v", result.DryRun, tt.expectedDryRun)
			}
		})
	}
}

func TestDoubleDashSeparator(t *testing.T) {
	// Everything after -- should be ignored for parsing
	tests := []struct {
		name             string
		args             []string
		expectedResource string
		expectedName     string
	}{
		{
			name:             "exec with -- separator",
			args:             []string{"exec", "nginx", "--", "/bin/sh", "-c", "ls"},
			expectedResource: "nginx",
			expectedName:     "", // /bin/sh should NOT be parsed as name
		},
		{
			name:             "exec with -it and -- separator",
			args:             []string{"exec", "-it", "nginx", "--", "/bin/bash"},
			expectedResource: "nginx",
			expectedName:     "",
		},
		{
			name:             "run with -- separator",
			args:             []string{"run", "test-pod", "--image=nginx", "--", "sleep", "3600"},
			expectedResource: "test-pod",
			expectedName:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Parse(tt.args)

			if result.Resource != tt.expectedResource {
				t.Errorf("Resource = %q, expected %q", result.Resource, tt.expectedResource)
			}
			if result.Name != tt.expectedName {
				t.Errorf("Name = %q, expected %q (args after -- should be ignored)", result.Name, tt.expectedName)
			}
		})
	}
}

func TestKustomizeFlag(t *testing.T) {
	// Bug: -k flag value was being parsed as resource/name
	tests := []struct {
		name             string
		args             []string
		expectedResource string
		expectedName     string
	}{
		{
			name:             "apply -k directory",
			args:             []string{"apply", "-k", "./kustomize"},
			expectedResource: "", // -k path should not be parsed as resource
			expectedName:     "",
		},
		{
			name:             "apply -k=directory",
			args:             []string{"apply", "-k=./overlays/prod"},
			expectedResource: "",
			expectedName:     "",
		},
		{
			name:             "apply --kustomize directory",
			args:             []string{"apply", "--kustomize", "./base"},
			expectedResource: "",
			expectedName:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Parse(tt.args)

			if result.Resource != tt.expectedResource {
				t.Errorf("Resource = %q, expected %q", result.Resource, tt.expectedResource)
			}
			if result.Name != tt.expectedName {
				t.Errorf("Name = %q, expected %q", result.Name, tt.expectedName)
			}
		})
	}
}

func TestFlagsWithValues(t *testing.T) {
	// Test that flag values are not parsed as resource/name
	tests := []struct {
		name             string
		args             []string
		expectedResource string
		expectedName     string
	}{
		{
			name:             "logs --tail value not parsed as name",
			args:             []string{"logs", "nginx", "--tail", "100"},
			expectedResource: "nginx",
			expectedName:     "",
		},
		{
			name:             "logs --since value not parsed as name",
			args:             []string{"logs", "nginx", "--since", "1h"},
			expectedResource: "nginx",
			expectedName:     "",
		},
		{
			name:             "port-forward --address value not parsed as resource",
			args:             []string{"port-forward", "--address", "0.0.0.0", "nginx", "8080:80"},
			expectedResource: "nginx",
			expectedName:     "8080:80",
		},
		{
			name:             "run --image value not parsed",
			args:             []string{"run", "nginx", "--image", "nginx:latest"},
			expectedResource: "nginx",
			expectedName:     "",
		},
		{
			name:             "scale --replicas value not parsed",
			args:             []string{"scale", "deployment/nginx", "--replicas", "3"},
			expectedResource: "deployment",
			expectedName:     "nginx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Parse(tt.args)

			if result.Resource != tt.expectedResource {
				t.Errorf("Resource = %q, expected %q", result.Resource, tt.expectedResource)
			}
			if result.Name != tt.expectedName {
				t.Errorf("Name = %q, expected %q", result.Name, tt.expectedName)
			}
		})
	}
}

func TestSetCommandSubcommands(t *testing.T) {
	// Bug: "set image" was parsing "image" as Resource instead of skipping the subcommand
	tests := []struct {
		name             string
		args             []string
		expectedResource string
		expectedName     string
	}{
		{
			name:             "set image deployment/nginx",
			args:             []string{"set", "image", "deployment/nginx", "nginx=nginx:1.16"},
			expectedResource: "deployment",
			expectedName:     "nginx",
		},
		{
			name:             "set env deployment nginx",
			args:             []string{"set", "env", "deployment", "nginx", "DEBUG=true"},
			expectedResource: "deployment",
			expectedName:     "nginx",
		},
		{
			name:             "set resources deployment/nginx",
			args:             []string{"set", "resources", "deployment/nginx", "--limits=cpu=200m"},
			expectedResource: "deployment",
			expectedName:     "nginx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Parse(tt.args)

			if result.Operation != "set" {
				t.Errorf("Operation = %q, expected %q", result.Operation, "set")
			}
			if result.Resource != tt.expectedResource {
				t.Errorf("Resource = %q, expected %q", result.Resource, tt.expectedResource)
			}
			if result.Name != tt.expectedName {
				t.Errorf("Name = %q, expected %q", result.Name, tt.expectedName)
			}
		})
	}
}

func TestGlobalFlagsWithEqualsSyntax(t *testing.T) {
	// Bug: --namespace=value before operation was causing findOperation
	// to incorrectly skip the operation (apply) because it thought there
	// was a separate value argument to skip
	tests := []struct {
		name              string
		args              []string
		expectedOperation string
		expectedNamespace string
		expectedFileInput []string
	}{
		{
			name:              "namespace= before apply -f",
			args:              []string{"--namespace=production", "apply", "-f", "deploy.yaml"},
			expectedOperation: "apply",
			expectedNamespace: "production",
			expectedFileInput: []string{"deploy.yaml"},
		},
		{
			name:              "context= before delete",
			args:              []string{"--context=prod-cluster", "delete", "pod", "nginx"},
			expectedOperation: "delete",
			expectedNamespace: "",
			expectedFileInput: nil,
		},
		{
			name:              "multiple = flags before operation",
			args:              []string{"--namespace=staging", "--context=dev", "apply", "-f", "svc.yaml"},
			expectedOperation: "apply",
			expectedNamespace: "staging",
			expectedFileInput: []string{"svc.yaml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Parse(tt.args)

			if result.Operation != tt.expectedOperation {
				t.Errorf("Operation = %q, expected %q", result.Operation, tt.expectedOperation)
			}
			if result.Namespace != tt.expectedNamespace {
				t.Errorf("Namespace = %q, expected %q", result.Namespace, tt.expectedNamespace)
			}
			if !reflect.DeepEqual(result.FileInputs, tt.expectedFileInput) {
				t.Errorf("FileInputs = %v, expected %v", result.FileInputs, tt.expectedFileInput)
			}
		})
	}
}

func TestParseFileInputs(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		fileInputs []string
		recursive  bool
	}{
		{
			name:       "single -f flag",
			args:       []string{"apply", "-f", "deploy.yaml"},
			fileInputs: []string{"deploy.yaml"},
			recursive:  false,
		},
		{
			name:       "multiple -f flags",
			args:       []string{"apply", "-f", "deploy.yaml", "-f", "service.yaml"},
			fileInputs: []string{"deploy.yaml", "service.yaml"},
			recursive:  false,
		},
		{
			name:       "-f= syntax",
			args:       []string{"apply", "-f=deploy.yaml"},
			fileInputs: []string{"deploy.yaml"},
			recursive:  false,
		},
		{
			name:       "--filename flag",
			args:       []string{"apply", "--filename", "deploy.yaml"},
			fileInputs: []string{"deploy.yaml"},
			recursive:  false,
		},
		{
			name:       "--filename= syntax",
			args:       []string{"apply", "--filename=deploy.yaml"},
			fileInputs: []string{"deploy.yaml"},
			recursive:  false,
		},
		{
			name:       "with -R flag",
			args:       []string{"apply", "-f", "./manifests/", "-R"},
			fileInputs: []string{"./manifests/"},
			recursive:  true,
		},
		{
			name:       "with --recursive flag",
			args:       []string{"apply", "-f", "./manifests/", "--recursive"},
			fileInputs: []string{"./manifests/"},
			recursive:  true,
		},
		{
			name:       "no file inputs",
			args:       []string{"get", "pods"},
			fileInputs: nil,
			recursive:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Parse(tt.args)

			if !reflect.DeepEqual(result.FileInputs, tt.fileInputs) {
				t.Errorf("FileInputs = %v, expected %v", result.FileInputs, tt.fileInputs)
			}

			if result.Recursive != tt.recursive {
				t.Errorf("Recursive = %v, expected %v", result.Recursive, tt.recursive)
			}
		})
	}
}
