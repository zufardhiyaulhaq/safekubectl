package parser

import (
	"strings"
)

// KubectlCommand represents a parsed kubectl command
type KubectlCommand struct {
	Operation     string   // e.g., delete, apply, get
	Resource      string   // e.g., pod, deployment, pod/nginx
	Name          string   // e.g., nginx (if separate from resource)
	Namespace     string   // from -n or --namespace flag
	Context       string   // from --context flag
	Args          []string // original arguments
	FileInputs    []string // paths/URLs from -f/--filename flags
	Recursive     bool     // -R/--recursive flag present
	AllNamespaces bool     // --all-namespaces or -A flag present
	DryRun        bool     // --dry-run flag present
}

// Node-scoped operations that don't have a namespace
var nodeScopedOperations = map[string]bool{
	"cordon":   true,
	"uncordon": true,
	"drain":    true,
	"taint":    true,
}

// Operations that use -f/--filename for file input
// Other operations use -f for other purposes (e.g., logs -f = follow)
var fileInputOperations = map[string]bool{
	"apply":    true,
	"delete":   true,
	"create":   true,
	"replace":  true,
	"patch":    true,
	"annotate": true,
	"label":    true,
	"scale":    true,
}

// Operations with subcommands (operation + subcommand + resource)
var operationsWithSubcommands = map[string][]string{
	"rollout": {"restart", "status", "undo", "history", "pause", "resume"},
	"config":  {"view", "use-context", "set-context", "delete-context", "get-contexts", "current-context"},
	"set":     {"image", "env", "resources", "selector", "serviceaccount", "subject"},
}

// Parse parses kubectl arguments and extracts command info
func Parse(args []string) *KubectlCommand {
	cmd := &KubectlCommand{
		Args:      args,
		Namespace: "", // empty means default namespace
	}

	if len(args) == 0 {
		return cmd
	}

	// First pass: find the operation to know how to interpret flags
	operation := findOperation(args)

	// Check if this operation uses -f for file input
	usesFileInput := fileInputOperations[operation]

	// Check if this operation has subcommands
	subcommands := operationsWithSubcommands[operation]
	hasSubcommand := len(subcommands) > 0

	// Skip global flags at the beginning
	i := 0
	for i < len(args) && strings.HasPrefix(args[i], "-") {
		// Handle file input flags (only for operations that use -f for files)
		if usesFileInput {
			if args[i] == "-f" || args[i] == "--filename" {
				if i+1 < len(args) {
					cmd.FileInputs = append(cmd.FileInputs, args[i+1])
					i += 2
					continue
				}
			} else if strings.HasPrefix(args[i], "-f=") {
				cmd.FileInputs = append(cmd.FileInputs, strings.TrimPrefix(args[i], "-f="))
				i++
				continue
			} else if strings.HasPrefix(args[i], "--filename=") {
				cmd.FileInputs = append(cmd.FileInputs, strings.TrimPrefix(args[i], "--filename="))
				i++
				continue
			}
		}

		// Handle recursive flag
		if args[i] == "-R" || args[i] == "--recursive" {
			cmd.Recursive = true
			i++
			continue
		}

		// Handle all-namespaces flag
		if args[i] == "-A" || args[i] == "--all-namespaces" {
			cmd.AllNamespaces = true
			i++
			continue
		}

		// Handle dry-run flag
		if args[i] == "--dry-run" || strings.HasPrefix(args[i], "--dry-run=") {
			cmd.DryRun = true
			i++
			continue
		}

		// Handle flags with values
		// If flag contains =, value is already embedded
		if strings.Contains(args[i], "=") {
			// Check for namespace/context with embedded value
			if strings.HasPrefix(args[i], "-n=") {
				cmd.Namespace = strings.TrimPrefix(args[i], "-n=")
			} else if strings.HasPrefix(args[i], "--namespace=") {
				cmd.Namespace = strings.TrimPrefix(args[i], "--namespace=")
			} else if strings.HasPrefix(args[i], "--context=") {
				cmd.Context = strings.TrimPrefix(args[i], "--context=")
			}
			i++
		} else if needsValue(args[i]) && i+1 < len(args) {
			// Check for namespace flag
			if args[i] == "-n" || args[i] == "--namespace" {
				cmd.Namespace = args[i+1]
			}
			// Check for context flag
			if args[i] == "--context" {
				cmd.Context = args[i+1]
			}
			i += 2
		} else {
			i++
		}
	}

	// First non-flag argument is the operation
	if i < len(args) {
		cmd.Operation = args[i]
		i++
	}

	// Handle operations with subcommands (e.g., rollout restart)
	// Skip the subcommand and parse resource from what follows
	if hasSubcommand && i < len(args) {
		nextArg := args[i]
		for _, sub := range subcommands {
			if nextArg == sub {
				i++ // Skip the subcommand
				break
			}
		}
	}

	// Parse remaining arguments for resource, name, namespace, and context
	for i < len(args) {
		arg := args[i]

		// Stop parsing at -- separator (everything after is command args, not kubectl args)
		if arg == "--" {
			break
		}

		// Handle file input flags (only for operations that use -f for files)
		if usesFileInput {
			if arg == "-f" || arg == "--filename" {
				if i+1 < len(args) {
					cmd.FileInputs = append(cmd.FileInputs, args[i+1])
					i += 2
					continue
				}
			} else if strings.HasPrefix(arg, "-f=") {
				cmd.FileInputs = append(cmd.FileInputs, strings.TrimPrefix(arg, "-f="))
				i++
				continue
			} else if strings.HasPrefix(arg, "--filename=") {
				cmd.FileInputs = append(cmd.FileInputs, strings.TrimPrefix(arg, "--filename="))
				i++
				continue
			}
		}

		// Handle recursive flag
		if arg == "-R" || arg == "--recursive" {
			cmd.Recursive = true
			i++
			continue
		}

		// Handle all-namespaces flag
		if arg == "-A" || arg == "--all-namespaces" {
			cmd.AllNamespaces = true
			i++
			continue
		}

		// Handle dry-run flag
		if arg == "--dry-run" || strings.HasPrefix(arg, "--dry-run=") {
			cmd.DryRun = true
			i++
			continue
		}

		// Handle namespace flag anywhere in args
		if arg == "-n" || arg == "--namespace" {
			if i+1 < len(args) {
				cmd.Namespace = args[i+1]
				i += 2
				continue
			}
		} else if strings.HasPrefix(arg, "-n=") {
			cmd.Namespace = strings.TrimPrefix(arg, "-n=")
			i++
			continue
		} else if strings.HasPrefix(arg, "--namespace=") {
			cmd.Namespace = strings.TrimPrefix(arg, "--namespace=")
			i++
			continue
		}

		// Handle context flag anywhere in args
		if arg == "--context" {
			if i+1 < len(args) {
				cmd.Context = args[i+1]
				i += 2
				continue
			}
		} else if strings.HasPrefix(arg, "--context=") {
			cmd.Context = strings.TrimPrefix(arg, "--context=")
			i++
			continue
		}

		// Skip other flags
		if strings.HasPrefix(arg, "-") {
			// If flag contains =, value is already embedded, don't skip next arg
			if strings.Contains(arg, "=") {
				i++
			} else if needsValue(arg) && i+1 < len(args) {
				i += 2
			} else {
				i++
			}
			continue
		}

		// This should be resource or resource/name
		if cmd.Resource == "" {
			// Check if it's resource/name format
			if strings.Contains(arg, "/") {
				parts := strings.SplitN(arg, "/", 2)
				cmd.Resource = parts[0]
				if len(parts) > 1 {
					cmd.Name = parts[1]
				}
			} else {
				cmd.Resource = arg
			}
		} else if cmd.Name == "" {
			// Second positional arg is the name
			cmd.Name = arg
		}
		i++
	}

	return cmd
}

// findOperation scans args to find the operation (first non-flag argument)
func findOperation(args []string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			// If flag contains =, value is embedded, don't skip next arg
			if strings.Contains(arg, "=") {
				continue
			}
			// Skip flag and its value if needed
			if needsValue(arg) && i+1 < len(args) {
				i++
			}
			continue
		}
		return arg
	}
	return ""
}

// needsValue returns true if the flag requires a value
func needsValue(flag string) bool {
	// Common kubectl flags that take values
	valueFlags := []string{
		"-n", "--namespace",
		"-f", "--filename",
		"-k", "--kustomize",
		"-l", "--selector",
		"-o", "--output",
		"--context",
		"--cluster",
		"--user",
		"--kubeconfig",
		"-c", "--container",
		"--field-selector",
		"--sort-by",
		"--template",
		"-p", "--patch",
		"--type",
		"--timeout",
		"--grace-period",
		"--tail",
		"--since",
		"--since-time",
		"--limit-bytes",
		"--address",
		"--image",
		"--replicas",
		"--for",
	}

	// Strip = suffix if present
	flag = strings.Split(flag, "=")[0]

	for _, vf := range valueFlags {
		if flag == vf {
			return true
		}
	}
	return false
}

// GetResourceDisplay returns a display string for the resource
func (k *KubectlCommand) GetResourceDisplay() string {
	if k.Resource == "" {
		return "<unknown>"
	}

	if k.Name != "" {
		return k.Resource + "/" + k.Name
	}
	return k.Resource
}

// GetNamespaceDisplay returns namespace or "default" if empty
func (k *KubectlCommand) GetNamespaceDisplay() string {
	if k.Namespace == "" {
		return "default"
	}
	return k.Namespace
}

// IsNodeScoped returns true if the operation is node-scoped (no namespace)
func (k *KubectlCommand) IsNodeScoped() bool {
	return nodeScopedOperations[k.Operation]
}
