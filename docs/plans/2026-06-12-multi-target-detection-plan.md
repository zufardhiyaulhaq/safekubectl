# Multi-Target Detection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Detect and report ALL objects targeted by a kubectl command (e.g. `delete secret a b c d`), not just the first one.

**Architecture:** Replace the single `Resource`/`Name` pair on `parser.KubectlCommand` with a `Targets []Target` slice built from kubectl's positional-argument rules (type + names, slash-form, comma-separated types). The checker carries a `Resources []string` display list, the prompt renders a tree of affected resources, and the audit log writes `resources=[...]`. Migration is additive-then-remove so every task compiles and stays green: Tasks 1–4 add the new fields alongside the old, Task 5 removes the old fields, Task 6 adds an end-to-end regression test.

**Tech Stack:** Go (stdlib only), `go test`. Spec: `docs/plans/2026-06-12-multi-target-detection-design.md`.

**⚠️ Commit policy:** The user asked not to commit without explicit approval. At each "Commit" step, ask the user before running `git commit` — or skip the commit steps and let the user review the full diff at the end.

---

## Background for implementers

safekubectl wraps kubectl: `main.go` parses args (`internal/parser`), checks danger (`internal/checker`), warns/prompts (`internal/prompt`), audit-logs (`internal/audit`), then execs kubectl. The bug: `internal/parser/parser.go` keeps only the first positional name — `delete secret a b c d` parses as `secret/a` and the warning/audit show one object.

kubectl positional forms this plan handles:

- `delete secret a b c` — one type, many names
- `delete pod/a pod/b secret/c` — slash-form targets
- `delete pods,services foo bar` — comma types; kubectl resolves each name against each type (cross product)
- `delete pods -l app=x` — type only, no names

Positional args containing `=` (taint specs `key=value:NoSchedule`, env vars `DEBUG=true`, `set image` pairs `nginx=nginx:1.16`) are never resource names — Kubernetes names cannot contain `=`. The parser ignores them when building targets.

Run all tests with: `go test ./... -v` (from the repo root).

---

### Task 1: Parser — `Target` type and multi-target parsing

**Files:**
- Modify: `internal/parser/parser.go`
- Test: `internal/parser/parser_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/parser/parser_test.go`:

```go
func TestParseMultipleTargets(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected []Target
	}{
		{
			name: "delete one type many names",
			args: []string{"delete", "secret", "cert-a", "cert-b", "cert-c", "cert-d"},
			expected: []Target{
				{Resource: "secret", Name: "cert-a"},
				{Resource: "secret", Name: "cert-b"},
				{Resource: "secret", Name: "cert-c"},
				{Resource: "secret", Name: "cert-d"},
			},
		},
		{
			name: "delete slash-form multiple targets",
			args: []string{"delete", "pod/a", "pod/b", "secret/c"},
			expected: []Target{
				{Resource: "pod", Name: "a"},
				{Resource: "pod", Name: "b"},
				{Resource: "secret", Name: "c"},
			},
		},
		{
			name: "comma-separated types without names",
			args: []string{"delete", "pods,services", "-l", "app=nginx"},
			expected: []Target{
				{Resource: "pods"},
				{Resource: "services"},
			},
		},
		{
			name: "comma types cross product with names",
			args: []string{"delete", "pods,services", "foo", "bar"},
			expected: []Target{
				{Resource: "pods", Name: "foo"},
				{Resource: "services", Name: "foo"},
				{Resource: "pods", Name: "bar"},
				{Resource: "services", Name: "bar"},
			},
		},
		{
			name: "flags interleaved between names",
			args: []string{"delete", "secret", "cert-a", "-n", "istio-system", "cert-b"},
			expected: []Target{
				{Resource: "secret", Name: "cert-a"},
				{Resource: "secret", Name: "cert-b"},
			},
		},
		{
			name:     "args after double dash ignored",
			args:     []string{"exec", "nginx", "--", "/bin/sh", "-c", "ls"},
			expected: []Target{{Resource: "nginx"}},
		},
		{
			name:     "type-only selector form",
			args:     []string{"delete", "pods", "-l", "app=nginx"},
			expected: []Target{{Resource: "pods"}},
		},
		{
			name:     "taint spec not treated as a name",
			args:     []string{"taint", "nodes", "node-1", "key=value:NoSchedule"},
			expected: []Target{{Resource: "nodes", Name: "node-1"}},
		},
		{
			name:     "file input has no targets",
			args:     []string{"apply", "-f", "deploy.yaml"},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Parse(tt.args)
			if !reflect.DeepEqual(result.Targets, tt.expected) {
				t.Errorf("Targets: got %v, expected %v", result.Targets, tt.expected)
			}
		})
	}
}

func TestGetResourceDisplays(t *testing.T) {
	tests := []struct {
		name     string
		cmd      *KubectlCommand
		expected []string
	}{
		{
			name: "multiple named targets",
			cmd: &KubectlCommand{Targets: []Target{
				{Resource: "secret", Name: "cert-a"},
				{Resource: "secret", Name: "cert-b"},
			}},
			expected: []string{"secret/cert-a", "secret/cert-b"},
		},
		{
			name:     "type-only target",
			cmd:      &KubectlCommand{Targets: []Target{{Resource: "pods"}}},
			expected: []string{"pods"},
		},
		{
			name:     "no targets",
			cmd:      &KubectlCommand{},
			expected: []string{"<unknown>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cmd.GetResourceDisplays()
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("GetResourceDisplays() = %v, expected %v", result, tt.expected)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/parser -v -run 'TestParseMultipleTargets|TestGetResourceDisplays'`
Expected: compile error — `undefined: Target`, `result.Targets undefined`.

- [ ] **Step 3: Implement multi-target parsing**

In `internal/parser/parser.go`:

3a. Add the `Target` type and a `Targets` field to `KubectlCommand` (keep `Resource` and `Name` for now — they are removed in Task 5):

```go
// Target is one object a kubectl command operates on
type Target struct {
	Resource string // e.g. "secret", "pod"
	Name     string // empty for type-only targets (e.g. delete pods -l app=x)
}
```

In the `KubectlCommand` struct, below the `Name` field, add:

```go
	Targets       []Target // all positional targets (resource type + optional name)
```

3b. In `Parse`, declare a collector before the second loop (`// Parse remaining arguments for resource, name, namespace, and context`):

```go
	var positionals []string
```

3c. Replace the resource/name assignment block at the end of that loop:

```go
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
```

with:

```go
		// Positional arg: part of the command's targets
		positionals = append(positionals, arg)
		i++
```

3d. After the loop (just before `return cmd`):

```go
	cmd.Targets = buildTargets(positionals)

	// Back-compat during migration: mirror the first target into the
	// legacy Resource/Name fields (removed in the cleanup task)
	if len(cmd.Targets) > 0 {
		cmd.Resource = cmd.Targets[0].Resource
		cmd.Name = cmd.Targets[0].Name
	}
```

3e. Add the builder and display helper at the bottom of the file:

```go
// buildTargets interprets positional args using kubectl's rules:
// slash-form (TYPE/NAME ...) or type-spec form (TYPE[,TYPE...] [NAME ...]).
// Args containing "=" are never targets (taint specs, env vars, set image
// pairs) and are ignored.
func buildTargets(positionals []string) []Target {
	var targetArgs []string
	for _, arg := range positionals {
		if strings.Contains(arg, "=") {
			continue
		}
		targetArgs = append(targetArgs, arg)
	}

	if len(targetArgs) == 0 {
		return nil
	}

	// Slash form: every arg is TYPE/NAME
	if strings.Contains(targetArgs[0], "/") {
		targets := make([]Target, 0, len(targetArgs))
		for _, arg := range targetArgs {
			parts := strings.SplitN(arg, "/", 2)
			t := Target{Resource: parts[0]}
			if len(parts) == 2 {
				t.Name = parts[1]
			}
			targets = append(targets, t)
		}
		return targets
	}

	// Type-spec form: first arg is TYPE[,TYPE...], remaining args are names.
	// kubectl resolves each name against each type (cross product).
	types := strings.Split(targetArgs[0], ",")
	names := targetArgs[1:]

	if len(names) == 0 {
		targets := make([]Target, 0, len(types))
		for _, typ := range types {
			targets = append(targets, Target{Resource: typ})
		}
		return targets
	}

	targets := make([]Target, 0, len(types)*len(names))
	for _, name := range names {
		for _, typ := range types {
			targets = append(targets, Target{Resource: typ, Name: name})
		}
	}
	return targets
}

// GetResourceDisplays returns a display string per target
func (k *KubectlCommand) GetResourceDisplays() []string {
	if len(k.Targets) == 0 {
		return []string{"<unknown>"}
	}
	displays := make([]string, 0, len(k.Targets))
	for _, t := range k.Targets {
		switch {
		case t.Resource == "":
			displays = append(displays, "<unknown>")
		case t.Name != "":
			displays = append(displays, t.Resource+"/"+t.Name)
		default:
			displays = append(displays, t.Resource)
		}
	}
	return displays
}
```

- [ ] **Step 4: Run the parser tests**

Run: `go test ./internal/parser -v`
Expected: ALL PASS — new tests pass, and every pre-existing test (which still asserts `Resource`/`Name`) passes via the back-compat mirror.

- [ ] **Step 5: Run the full suite**

Run: `go test ./...`
Expected: PASS (no other package touched yet).

- [ ] **Step 6: Commit (ask the user first — see commit policy)**

```bash
git add internal/parser/parser.go internal/parser/parser_test.go
git commit -m "feat(parser): parse all positional targets into Targets slice"
```

---

### Task 2: Checker — carry all target displays in `CheckResult`

**Files:**
- Modify: `internal/checker/checker.go`
- Test: `internal/checker/checker_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/checker/checker_test.go` (add `"reflect"` to the imports):

```go
func TestCheckResultMultipleResources(t *testing.T) {
	cfg := &config.Config{
		Mode:                config.ModeConfirm,
		DangerousOperations: []string{"delete"},
		ProtectedNamespaces: []string{},
		ProtectedClusters:   []string{},
	}

	chk := New(cfg)
	cmd := parser.Parse([]string{"delete", "secret", "cert-a", "cert-b", "-n", "istio-system"})
	result := chk.Check(cmd, "prod-cluster")

	expected := []string{"secret/cert-a", "secret/cert-b"}
	if !reflect.DeepEqual(result.Resources, expected) {
		t.Errorf("Resources: got %v, expected %v", result.Resources, expected)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/checker -v -run TestCheckResultMultipleResources`
Expected: compile error — `result.Resources undefined`.

- [ ] **Step 3: Implement**

In `internal/checker/checker.go`:

In the `CheckResult` struct, below `Resource string`, add:

```go
	Resources            []string // display string per target, e.g. ["secret/a", "secret/b"]
```

In `Check`, in the `result := &CheckResult{...}` literal, below `Resource: cmd.GetResourceDisplay(),` add:

```go
		Resources:       cmd.GetResourceDisplays(),
```

(`Resource` stays populated until Task 5.)

- [ ] **Step 4: Run the checker tests**

Run: `go test ./internal/checker -v`
Expected: ALL PASS.

- [ ] **Step 5: Commit (ask the user first — see commit policy)**

```bash
git add internal/checker/checker.go internal/checker/checker_test.go
git commit -m "feat(checker): expose all target displays via CheckResult.Resources"
```

---

### Task 3: Prompt — render all targets as a tree

**Files:**
- Modify: `internal/prompt/prompt.go`
- Test: `internal/prompt/prompt_test.go`

- [ ] **Step 1: Update and add tests**

In `internal/prompt/prompt_test.go`, replace `TestDisplayWarningTo` and `TestDisplayWarningToWithEmptyFields` entirely with:

```go
func TestDisplayWarningTo(t *testing.T) {
	result := &checker.CheckResult{
		Operation: "delete",
		Resources: []string{"pod/nginx"},
		Namespace: "production",
		Cluster:   "prod-cluster",
	}
	args := []string{"delete", "pod", "nginx", "-n", "production"}

	var buf bytes.Buffer
	DisplayWarningTo(&buf, result, args)
	output := buf.String()

	// Check that all expected elements are present
	expectedParts := []string{
		"DANGEROUS OPERATION DETECTED",
		"Operation:",
		"delete",
		"Resources affected:",
		"pod/nginx",
		"Namespace:",
		"production",
		"Cluster:",
		"prod-cluster",
		"Command:",
		"kubectl delete pod nginx -n production",
	}

	for _, part := range expectedParts {
		if !strings.Contains(output, part) {
			t.Errorf("expected output to contain %q, got:\n%s", part, output)
		}
	}

	// Check tree structure characters
	if !strings.Contains(output, "├──") {
		t.Error("expected output to contain tree structure '├──'")
	}
	if !strings.Contains(output, "└──") {
		t.Error("expected output to contain tree structure '└──'")
	}
}

func TestDisplayWarningToMultipleResources(t *testing.T) {
	result := &checker.CheckResult{
		Operation: "delete",
		Resources: []string{"secret/cert-a", "secret/cert-b", "secret/cert-c"},
		Namespace: "istio-system",
		Cluster:   "prod-cluster",
	}
	args := []string{"delete", "secret", "cert-a", "cert-b", "cert-c"}

	var buf bytes.Buffer
	DisplayWarningTo(&buf, result, args)
	output := buf.String()

	for _, r := range result.Resources {
		if !strings.Contains(output, r) {
			t.Errorf("expected output to contain %q, got:\n%s", r, output)
		}
	}

	// Non-last entries branch with ├──, the last closes with └──
	if !strings.Contains(output, "│   ├── secret/cert-a") {
		t.Errorf("expected tree branch for first resource, got:\n%s", output)
	}
	if !strings.Contains(output, "│   └── secret/cert-c") {
		t.Errorf("expected closing branch for last resource, got:\n%s", output)
	}
}

func TestDisplayWarningToWithEmptyFields(t *testing.T) {
	result := &checker.CheckResult{
		Operation: "",
		Resources: nil,
		Namespace: "",
		Cluster:   "",
	}
	args := []string{}

	var buf bytes.Buffer
	DisplayWarningTo(&buf, result, args)
	output := buf.String()

	// Should still contain the header and a placeholder resource
	if !strings.Contains(output, "DANGEROUS OPERATION DETECTED") {
		t.Error("expected output to contain warning header")
	}
	if !strings.Contains(output, "<unknown>") {
		t.Error("expected output to contain <unknown> for empty resources")
	}
}
```

Then run `grep -n 'Resource' internal/prompt/prompt_test.go` and in every **other** test that builds a `checker.CheckResult` literal (the all-namespaces and node-scoped warning tests), apply these two mechanical replacements:
- field literal `Resource:  "X",` → `Resources: []string{"X"},`
- expected substring `"Resource:"` → `"Resources affected:"` (if present)

Do **not** change `ResourceCheckResult` literals or `DisplayResourceWarningTo` tests — those belong to the file-based path and are untouched.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/prompt -v`
Expected: FAIL — output lacks `Resources affected:` (old single-line `Resource:` rendering still in place).

- [ ] **Step 3: Implement tree rendering**

In `internal/prompt/prompt.go`, replace the body of `DisplayWarningTo` with:

```go
func DisplayWarningTo(w io.Writer, result *checker.CheckResult, args []string) {
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s%s  DANGEROUS OPERATION DETECTED%s\n", colorYellow, warningIcon(), colorReset)
	fmt.Fprintf(w, "├── Operation: %s%s%s\n", colorRed, result.Operation, colorReset)
	// Show namespace info based on scope
	if result.IsAllNamespaces {
		fmt.Fprintf(w, "├── Namespace: %s⚠ ALL NAMESPACES%s\n", colorRed, colorReset)
	} else if !result.IsNodeScoped {
		fmt.Fprintf(w, "├── Namespace: %s\n", result.Namespace)
	}
	fmt.Fprintf(w, "├── Cluster:   %s\n", result.Cluster)
	fmt.Fprintln(w, "├── Resources affected:")
	resources := result.Resources
	if len(resources) == 0 {
		resources = []string{"<unknown>"}
	}
	for i, r := range resources {
		prefix := "│   ├──"
		if i == len(resources)-1 {
			prefix = "│   └──"
		}
		fmt.Fprintf(w, "%s %s\n", prefix, r)
	}
	fmt.Fprintf(w, "└── Command:   kubectl %s\n", strings.Join(args, " "))
	fmt.Fprintln(w)
}
```

- [ ] **Step 4: Run the prompt tests**

Run: `go test ./internal/prompt -v`
Expected: ALL PASS.

- [ ] **Step 5: Run the full suite**

Run: `go test ./...`
Expected: PASS. (main_test.go only asserts the `DANGEROUS OPERATION DETECTED` header, which is unchanged.)

- [ ] **Step 6: Commit (ask the user first — see commit policy)**

```bash
git add internal/prompt/prompt.go internal/prompt/prompt_test.go
git commit -m "feat(prompt): render all affected resources as a tree in warnings"
```

---

### Task 4: Audit — log all targets

**Files:**
- Modify: `internal/audit/audit.go`
- Test: `internal/audit/audit_test.go`

- [ ] **Step 1: Update and add tests**

Append to `internal/audit/audit_test.go`:

```go
func TestLogMultipleResources(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	cfg := &config.Config{
		Audit: config.AuditConfig{
			Enabled: true,
			Path:    logPath,
		},
	}

	logger := New(cfg)
	result := &checker.CheckResult{
		Operation: "delete",
		Resources: []string{"secret/cert-a", "secret/cert-b"},
		Namespace: "istio-system",
		Cluster:   "prod-cluster",
	}

	err := logger.Log(result, []string{"delete", "secret", "cert-a", "cert-b"}, true, true)
	if err != nil {
		t.Fatalf("Log() returned error: %v", err)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "resources=[secret/cert-a,secret/cert-b]") {
		t.Errorf("log entry missing resources list, got:\n%s", string(content))
	}
}
```

Then run `grep -n 'Resource\|resource=' internal/audit/audit_test.go` and in every existing test that uses `checker.CheckResult` (NOT `ResourceCheckResult`), apply these mechanical replacements:
- field literal `Resource:  "pod/nginx",` → `Resources: []string{"pod/nginx"},` (same pattern for `"deployment/web"`, `"deployment/nginx"`, etc.)
- expected substring `"resource=pod/nginx",` → `"resources=[pod/nginx]",` (same pattern for other values)

Leave `LogResources`/`ResourceCheckResult` tests untouched.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/audit -v`
Expected: FAIL — `TestLogMultipleResources` and updated assertions miss `resources=[...]` (old `resource=` key still written).

- [ ] **Step 3: Implement**

In `internal/audit/audit.go`, in `Log()`, replace the entry format block:

```go
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
```

with:

```go
	entry := fmt.Sprintf("[%s] %s | operation=%s resources=[%s] namespace=%s cluster=%s confirmed=%t command=\"%s\"\n",
		timestamp,
		status,
		result.Operation,
		strings.Join(result.Resources, ","),
		result.Namespace,
		result.Cluster,
		confirmed,
		strings.Join(args, " "),
	)
```

- [ ] **Step 4: Run the audit tests**

Run: `go test ./internal/audit -v`
Expected: ALL PASS.

- [ ] **Step 5: Commit (ask the user first — see commit policy)**

```bash
git add internal/audit/audit.go internal/audit/audit_test.go
git commit -m "feat(audit): log all targets as resources=[...]"
```

---

### Task 5: Cleanup — remove the legacy single Resource/Name fields

No new behavior; this removes the migration scaffolding. Tests are converted from `Resource`/`Name` assertions to `Targets`.

**Files:**
- Modify: `internal/parser/parser.go`
- Modify: `internal/parser/parser_test.go`
- Modify: `internal/checker/checker.go`
- Modify: `internal/checker/checker_test.go`

- [ ] **Step 1: Convert the parser tests to assert Targets**

In `internal/parser/parser_test.go`:

1a. Add this helper near the top of the file:

```go
// firstTarget returns the first parsed target, or a zero Target if none
func firstTarget(cmd *KubectlCommand) Target {
	if len(cmd.Targets) == 0 {
		return Target{}
	}
	return cmd.Targets[0]
}
```

1b. In `TestParse`, replace each entry's `Resource:`/`Name:` expected fields with `Targets:`, per this exact mapping (entries not listed have `Resource: ""`/`Name: ""` today and get no `Targets` field, i.e. nil):

| Test entry name | `Targets:` value |
|---|---|
| simple delete pod | `[]Target{{Resource: "pod", Name: "nginx"}}` |
| delete with resource/name format | `[]Target{{Resource: "pod", Name: "nginx"}}` |
| delete with namespace flag -n | `[]Target{{Resource: "pod", Name: "nginx"}}` |
| delete with namespace flag --namespace | `[]Target{{Resource: "pod", Name: "nginx"}}` |
| delete with namespace flag -n=value | `[]Target{{Resource: "pod", Name: "nginx"}}` |
| delete with namespace flag --namespace=value | `[]Target{{Resource: "pod", Name: "nginx"}}` |
| namespace flag before operation | `[]Target{{Resource: "pod", Name: "nginx"}}` |
| get pods (safe operation) | `[]Target{{Resource: "pods"}}` |
| apply with file flag | nil (omit field) |
| apply with file and namespace | nil (omit field) |
| exec command | `[]Target{{Resource: "nginx"}}` |
| rollout restart deployment | `[]Target{{Resource: "deployment", Name: "nginx"}}` |
| drain node | `[]Target{{Resource: "node-1"}}` |
| cordon node | `[]Target{{Resource: "node-1"}}` |
| taint node | `[]Target{{Resource: "nodes", Name: "node-1"}}` |
| patch deployment | `[]Target{{Resource: "deployment", Name: "nginx"}}` |
| edit configmap | `[]Target{{Resource: "configmap", Name: "my-config"}}` |
| empty args | nil (omit field) |
| with output flag | `[]Target{{Resource: "pods"}}` |
| with selector flag | `[]Target{{Resource: "pods"}}` |
| with context flag before operation | `[]Target{{Resource: "pod", Name: "nginx"}}` |
| with context flag after operation | `[]Target{{Resource: "pod", Name: "nginx"}}` |
| with context= flag | `[]Target{{Resource: "pod", Name: "nginx"}}` |
| with context and namespace | `[]Target{{Resource: "pod", Name: "nginx"}}` |

1c. In `TestParse`'s assertion loop, replace the `Resource` and `Name` comparison blocks:

```go
			if result.Resource != tt.expected.Resource {
				t.Errorf("Resource: got %q, expected %q", result.Resource, tt.expected.Resource)
			}

			if result.Name != tt.expected.Name {
				t.Errorf("Name: got %q, expected %q", result.Name, tt.expected.Name)
			}
```

with:

```go
			if !reflect.DeepEqual(result.Targets, tt.expected.Targets) {
				t.Errorf("Targets: got %v, expected %v", result.Targets, tt.expected.Targets)
			}
```

1d. In `TestLogsWithFollowFlag`, `TestRolloutRestartParsing`, `TestDoubleDashSeparator`, `TestKustomizeFlag`, `TestFlagsWithValues`, and `TestSetCommandSubcommands`, replace every read of `result.Resource` with `firstTarget(result).Resource` and every read of `result.Name` with `firstTarget(result).Name`. The expected values are unchanged (the zero `Target` covers the `""` expectations).

1e. Delete `TestGetResourceDisplay` entirely (superseded by `TestGetResourceDisplays` from Task 1). In that function's old table, the `cmd` literals used `Resource:`/`Name:` fields — make sure no other test still constructs `KubectlCommand{Resource: ...}` or `{Name: ...}` literals (`grep -n 'Resource:\|Name:' internal/parser/parser_test.go` should only show `Target{...}` literals afterward).

- [ ] **Step 2: Convert the checker test**

In `internal/checker/checker_test.go`, in `TestCheckResultFields`, replace:

```go
	if result.Resource != "pod/nginx" {
		t.Errorf("Resource: got %q, expected %q", result.Resource, "pod/nginx")
	}
```

with:

```go
	if !reflect.DeepEqual(result.Resources, []string{"pod/nginx"}) {
		t.Errorf("Resources: got %v, expected %v", result.Resources, []string{"pod/nginx"})
	}
```

- [ ] **Step 3: Run the converted tests before removing anything**

Run: `go test ./internal/parser ./internal/checker -v`
Expected: ALL PASS (the converted tests exercise the new fields, which are already populated; this confirms the conversion is correct before the legacy fields are removed).

- [ ] **Step 4: Remove the legacy fields**

4a. `internal/parser/parser.go`:
- Remove the `Resource string` and `Name string` fields from `KubectlCommand`.
- Remove the back-compat mirror block added in Task 1 step 3d (the `if len(cmd.Targets) > 0 { cmd.Resource = ...; cmd.Name = ... }` lines), keeping `cmd.Targets = buildTargets(positionals)`.
- Remove the `GetResourceDisplay() string` method entirely.

4b. `internal/checker/checker.go`:
- Remove the `Resource string` field from `CheckResult` (keep `Resources []string`).
- Remove `Resource: cmd.GetResourceDisplay(),` from the `result := &CheckResult{...}` literal in `Check` (keep `Resources:`).

- [ ] **Step 5: Verify nothing references the removed fields**

Run: `go build ./... && go vet ./...`
Expected: clean. If anything still references `cmd.Resource`, `cmd.Name`, `GetResourceDisplay`, or `CheckResult.Resource`, the compiler lists it — convert it the same way as above.

- [ ] **Step 6: Run the full suite**

Run: `go test ./... -v`
Expected: ALL PASS.

- [ ] **Step 7: Commit (ask the user first — see commit policy)**

```bash
git add internal/parser internal/checker
git commit -m "refactor: remove legacy single Resource/Name fields"
```

---

### Task 6: End-to-end regression test for the original bug

**Files:**
- Test: `main_test.go`

- [ ] **Step 1: Write the test**

Append to `main_test.go`:

```go
func TestRunDangerousMultipleTargets(t *testing.T) {
	// Regression: "delete secret a b c d" must warn about ALL four secrets,
	// not just the first one
	executed := false
	var stdout bytes.Buffer

	runner := &Runner{
		stdin:  strings.NewReader("y\n"),
		stdout: &stdout,
		stderr: &bytes.Buffer{},
		getCluster: func() string {
			return "test-cluster"
		},
		getContextNamespace: func(ctx string) string { return "istio-system" },
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

	err := runner.Run([]string{"delete", "secret", "cert-a", "cert-b", "cert-c", "cert-d"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !executed {
		t.Error("expected kubectl to be executed after confirmation")
	}

	output := stdout.String()
	for _, want := range []string{"secret/cert-a", "secret/cert-b", "secret/cert-c", "secret/cert-d"} {
		if !strings.Contains(output, want) {
			t.Errorf("warning missing %q, got:\n%s", want, output)
		}
	}
}
```

- [ ] **Step 2: Run the test**

Run: `go test . -v -run TestRunDangerousMultipleTargets`
Expected: PASS (Tasks 1–3 already implement the behavior; this test pins it end-to-end).

- [ ] **Step 3: Run the full suite with coverage**

Run: `go test ./... -cover`
Expected: ALL PASS.

- [ ] **Step 4: Manual smoke test**

```bash
go build -o safekubectl .
./safekubectl delete secret cert-a cert-b cert-c cert-d --dry-run=client
```
Expected: passes straight through to kubectl (dry-run is safe). Then, without `--dry-run` against a non-critical context, the warning must list all four secrets under "Resources affected:" before prompting. Answer `n` to abort.

- [ ] **Step 5: Commit (ask the user first — see commit policy)**

```bash
git add main_test.go
git commit -m "test: end-to-end regression for multi-target warning"
```
