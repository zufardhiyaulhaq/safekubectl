# JSON Audit Output Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an opt-in JSON (JSON Lines) audit output format, selectable via config, while keeping text as the default.

**Architecture:** Introduce one JSON-tagged `Entry` struct as the single audit record. `Log` (CLI commands) and `LogResources` (file-based `-f` commands) each build an `Entry` and delegate to a shared private `writeEntry`, which owns file handling and picks `formatText` or `formatJSON` based on `config.Audit.Format`. This both adds JSON and removes the duplicated boilerplate the two methods share today.

**Tech Stack:** Go (stdlib only: `encoding/json`, `os`, `fmt`, `strings`, `time`, `path/filepath`), `go test`. Spec: `docs/plans/2026-06-14-json-audit-output-design.md`.

**⚠️ Commit policy:** The user asked not to commit without explicit approval. At each "Commit" step, ask the user before running `git commit` — or skip the commit steps and let the user review the full diff at the end.

---

## Background for implementers

safekubectl wraps kubectl and audit-logs dangerous commands. `internal/audit/audit.go` has two public methods:

- `Log(result *checker.CheckResult, args []string, confirmed, executed bool) error` — CLI path. Has a top-level namespace.
- `LogResources(result *checker.ResourceCheckResult, args []string, confirmed, executed bool) error` — file-based `-f` path. Namespace is per-resource, baked into each resource string as `Kind/Name@namespace`.

Both currently: check `config.Audit.Enabled`, `os.MkdirAll` the log dir, open the file `O_APPEND|O_CREATE|O_WRONLY` (0644), compute an RFC3339 timestamp and `EXECUTED`/`DENIED` status, format a `key=value` line, and append it. The boilerplate is duplicated; only the line format differs.

**Current exact line formats (must understand for backwards-compat):**

CLI (`Log`):
```
[<ts>] <status> | operation=<op> resources=[<r1,r2>] namespace=<ns> cluster=<cluster> confirmed=<bool> command="<args>"
```
File (`LogResources`):
```
[<ts>] <status> | operation=<op> cluster=<cluster> resources=[<Kind/Name@ns,...>] confirmed=<bool> command="<args>"
```

After this change, **both** paths render through one unified text layout identical to the current CLI layout. The CLI text line is therefore byte-for-byte unchanged; the file-based text line changes (gains an empty `namespace=`, and `cluster` moves to after `namespace`). The existing file-based tests assert only substrings (`operation=apply`, `cluster=prod-cluster`, `Deployment/nginx@production`, `confirmed=true`, `DENIED`) that all survive the reorder, so they keep passing.

**Important:** the command field must use literal quotes `command="%s"` (NOT `%q`). `%q` would escape embedded quotes and break `TestLogWithSpecialCharactersInCommand`, which expects `command="patch deployment nginx -p {"spec":{"replicas":3}}"` with unescaped inner quotes.

**Format resolution rule:** only the exact value `"json"` selects JSON. Empty string or any other value resolves to text. This matters because every existing audit test (and any pre-existing config file) builds `config.AuditConfig` without a `Format`, leaving it `""` → text.

Run all tests with: `go test ./... -v` (from the repo root). Single package: `go test ./internal/audit -v`.

---

### Task 1: Config — add the `Format` field

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `config.example.yaml`

- [ ] **Step 1: Write the failing tests**

Append to `internal/config/config_test.go`:

```go
func TestAuditFormatDefault(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Audit.Format != "text" {
		t.Errorf("expected default audit format %q, got %q", "text", cfg.Audit.Format)
	}
}

func TestAuditFormatFromYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	content := "audit:\n  enabled: true\n  format: json\n"
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	os.Setenv("SAFEKUBECTL_CONFIG", configPath)
	defer os.Unsetenv("SAFEKUBECTL_CONFIG")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if cfg.Audit.Format != "json" {
		t.Errorf("expected audit format %q, got %q", "json", cfg.Audit.Format)
	}
}
```

(`config_test.go` already imports `os`, `path/filepath`, and `testing`.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config -v -run 'TestAuditFormatDefault|TestAuditFormatFromYAML'`
Expected: compile error — `cfg.Audit.Format undefined (type AuditConfig has no field or method Format)`.

- [ ] **Step 3: Add the field and default**

In `internal/config/config.go`, change the `AuditConfig` struct from:

```go
type AuditConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}
```

to:

```go
type AuditConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
	Format  string `yaml:"format"` // "text" (default) or "json"
}
```

In `DefaultConfig`, change the `Audit` literal from:

```go
		Audit: AuditConfig{
			Enabled: false,
			Path:    filepath.Join(homeDir, ".safekubectl", "audit.log"),
		},
```

to:

```go
		Audit: AuditConfig{
			Enabled: false,
			Path:    filepath.Join(homeDir, ".safekubectl", "audit.log"),
			Format:  "text",
		},
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config -v`
Expected: ALL PASS (including the existing `TestDefaultConfig`, which does not assert `Format`).

- [ ] **Step 5: Document the field in the example config**

In `config.example.yaml`, change the audit block from:

```yaml
# Audit logging configuration
audit:
  enabled: false
  path: ~/.safekubectl/audit.log
```

to:

```yaml
# Audit logging configuration
audit:
  enabled: false
  path: ~/.safekubectl/audit.log
  # Output format: "text" (default) or "json" (JSON Lines, one object per line)
  format: text
```

- [ ] **Step 6: Commit (ask the user first — see commit policy)**

```bash
git add internal/config/config.go internal/config/config_test.go config.example.yaml
git commit -m "feat(config): add audit.format field (text|json)"
```

---

### Task 2: Audit — `Entry` type and pure formatters

Add the record type and the two stateless formatters with unit tests. Nothing is wired into `Log`/`LogResources` yet, so the existing methods and their tests stay untouched and green.

**Files:**
- Modify: `internal/audit/audit.go`
- Test: `internal/audit/audit_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/audit/audit_test.go`:

```go
func TestFormatText(t *testing.T) {
	e := Entry{
		Timestamp: "2026-06-14T10:38:14Z",
		Status:    "EXECUTED",
		Operation: "delete",
		Resources: []string{"secret/cert-a", "secret/cert-b"},
		Namespace: "istio-system",
		Cluster:   "prod-cluster",
		Confirmed: true,
		Command:   "delete secret cert-a cert-b -n istio-system",
	}

	got := formatText(e)
	want := `[2026-06-14T10:38:14Z] EXECUTED | operation=delete resources=[secret/cert-a,secret/cert-b] namespace=istio-system cluster=prod-cluster confirmed=true command="delete secret cert-a cert-b -n istio-system"`

	if got != want {
		t.Errorf("formatText():\n got: %s\nwant: %s", got, want)
	}
}

func TestFormatTextEmptyNamespace(t *testing.T) {
	e := Entry{
		Timestamp: "2026-06-14T10:38:14Z",
		Status:    "EXECUTED",
		Operation: "apply",
		Resources: []string{"Deployment/nginx@istio-system"},
		Namespace: "",
		Cluster:   "prod-cluster",
		Confirmed: true,
		Command:   "apply -f deploy.yaml",
	}

	got := formatText(e)
	// file-based path: empty namespace renders as "namespace= "
	if !strings.Contains(got, "resources=[Deployment/nginx@istio-system] namespace= cluster=prod-cluster") {
		t.Errorf("formatText() with empty namespace, got: %s", got)
	}
}

func TestFormatTextPreservesCommandQuotes(t *testing.T) {
	e := Entry{
		Timestamp: "2026-06-14T10:38:14Z",
		Status:    "EXECUTED",
		Operation: "patch",
		Resources: []string{"deployment/nginx"},
		Namespace: "default",
		Cluster:   "test-cluster",
		Confirmed: true,
		Command:   `patch deployment nginx -p {"spec":{"replicas":3}}`,
	}

	got := formatText(e)
	if !strings.Contains(got, `command="patch deployment nginx -p {"spec":{"replicas":3}}"`) {
		t.Errorf("formatText() must preserve unescaped quotes, got: %s", got)
	}
}

func TestFormatJSON(t *testing.T) {
	e := Entry{
		Timestamp: "2026-06-14T10:38:14Z",
		Status:    "DENIED",
		Operation: "delete",
		Resources: []string{"secret/cert-a", "secret/cert-b"},
		Namespace: "istio-system",
		Cluster:   "prod-cluster",
		Confirmed: false,
		Command:   "delete secret cert-a cert-b -n istio-system",
	}

	got, err := formatJSON(e)
	if err != nil {
		t.Fatalf("formatJSON() returned error: %v", err)
	}

	// Must be a single line (no embedded newline)
	if strings.Contains(got, "\n") {
		t.Errorf("formatJSON() must not contain a newline, got: %s", got)
	}

	var decoded Entry
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("formatJSON() produced invalid JSON: %v\n%s", err, got)
	}

	if decoded.Status != "DENIED" {
		t.Errorf("status: got %q, want %q", decoded.Status, "DENIED")
	}
	if decoded.Operation != "delete" {
		t.Errorf("operation: got %q, want %q", decoded.Operation, "delete")
	}
	if len(decoded.Resources) != 2 || decoded.Resources[0] != "secret/cert-a" || decoded.Resources[1] != "secret/cert-b" {
		t.Errorf("resources: got %v", decoded.Resources)
	}
	if decoded.Namespace != "istio-system" {
		t.Errorf("namespace: got %q", decoded.Namespace)
	}
	if decoded.Cluster != "prod-cluster" {
		t.Errorf("cluster: got %q", decoded.Cluster)
	}
	if decoded.Confirmed {
		t.Errorf("confirmed: got true, want false")
	}
	if decoded.Command != "delete secret cert-a cert-b -n istio-system" {
		t.Errorf("command: got %q", decoded.Command)
	}
}

func TestFormatJSONKeys(t *testing.T) {
	e := Entry{Timestamp: "t", Status: "EXECUTED", Operation: "delete"}
	got, err := formatJSON(e)
	if err != nil {
		t.Fatalf("formatJSON() error: %v", err)
	}
	for _, key := range []string{
		`"timestamp"`, `"status"`, `"operation"`, `"resources"`,
		`"namespace"`, `"cluster"`, `"confirmed"`, `"command"`,
	} {
		if !strings.Contains(got, key) {
			t.Errorf("formatJSON() missing key %s, got: %s", key, got)
		}
	}
}
```

(`audit_test.go` already imports `strings` and `testing`. You will add `encoding/json` to its imports — add the line `"encoding/json"` to the test file's import block.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/audit -v -run 'TestFormat'`
Expected: compile error — `undefined: Entry`, `undefined: formatText`, `undefined: formatJSON`.

- [ ] **Step 3: Add the Entry type, formatters, and import**

In `internal/audit/audit.go`, add `"encoding/json"` to the import block so it reads:

```go
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
```

Then add, just below the `New` function:

```go
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/audit -v`
Expected: ALL PASS — new formatter tests pass; all existing `Log`/`LogResources` tests still pass (they are untouched).

- [ ] **Step 5: Commit (ask the user first — see commit policy)**

```bash
git add internal/audit/audit.go internal/audit/audit_test.go
git commit -m "feat(audit): add Entry type and text/json formatters"
```

---

### Task 3: Audit — shared `writeEntry` and refactor `Log`/`LogResources`

Wire the formatters in: introduce `writeEntry` (the single owner of file handling + format dispatch) and rewrite both public methods to build an `Entry` and delegate. Add JSON integration tests and a unified-text test for the file-based path.

**Files:**
- Modify: `internal/audit/audit.go`
- Test: `internal/audit/audit_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/audit/audit_test.go`:

```go
func TestLogJSONFormat(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	cfg := &config.Config{
		Audit: config.AuditConfig{
			Enabled: true,
			Path:    logPath,
			Format:  "json",
		},
	}

	logger := New(cfg)
	result := &checker.CheckResult{
		Operation: "delete",
		Resources: []string{"secret/cert-a", "secret/cert-b"},
		Namespace: "istio-system",
		Cluster:   "prod-cluster",
	}

	if err := logger.Log(result, []string{"delete", "secret", "cert-a", "cert-b", "-n", "istio-system"}, true, true); err != nil {
		t.Fatalf("Log() returned error: %v", err)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	line := strings.TrimSpace(string(content))
	var e Entry
	if err := json.Unmarshal([]byte(line), &e); err != nil {
		t.Fatalf("log line is not valid JSON: %v\n%s", err, line)
	}

	if e.Status != "EXECUTED" {
		t.Errorf("status: got %q, want EXECUTED", e.Status)
	}
	if e.Operation != "delete" {
		t.Errorf("operation: got %q, want delete", e.Operation)
	}
	if len(e.Resources) != 2 {
		t.Errorf("resources: got %v, want 2 entries", e.Resources)
	}
	if e.Namespace != "istio-system" {
		t.Errorf("namespace: got %q, want istio-system", e.Namespace)
	}
	if !e.Confirmed {
		t.Errorf("confirmed: got false, want true")
	}
}

func TestLogResourcesJSONFormat(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	cfg := &config.Config{
		Audit: config.AuditConfig{
			Enabled: true,
			Path:    logPath,
			Format:  "json",
		},
	}

	logger := New(cfg)
	result := &checker.ResourceCheckResult{
		Operation: "apply",
		Cluster:   "prod-cluster",
		Resources: []manifest.Resource{
			{Kind: "Deployment", Name: "nginx", Namespace: "production"},
		},
	}

	if err := logger.LogResources(result, []string{"apply", "-f", "deploy.yaml"}, true, true); err != nil {
		t.Fatalf("LogResources() returned error: %v", err)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	line := strings.TrimSpace(string(content))
	var e Entry
	if err := json.Unmarshal([]byte(line), &e); err != nil {
		t.Fatalf("log line is not valid JSON: %v\n%s", err, line)
	}

	// File-based path: top-level namespace is empty, ns is baked into the resource string
	if e.Namespace != "" {
		t.Errorf("namespace: got %q, want empty for file-based path", e.Namespace)
	}
	if len(e.Resources) != 1 || e.Resources[0] != "Deployment/nginx@production" {
		t.Errorf("resources: got %v, want [Deployment/nginx@production]", e.Resources)
	}
}

func TestLogResourcesTextUnified(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	cfg := &config.Config{
		Audit: config.AuditConfig{
			Enabled: true,
			Path:    logPath,
			Format:  "text",
		},
	}

	logger := New(cfg)
	result := &checker.ResourceCheckResult{
		Operation: "apply",
		Cluster:   "prod-cluster",
		Resources: []manifest.Resource{
			{Kind: "Deployment", Name: "nginx", Namespace: "production"},
		},
	}

	if err := logger.LogResources(result, []string{"apply", "-f", "deploy.yaml"}, true, true); err != nil {
		t.Fatalf("LogResources() returned error: %v", err)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	// Unified layout: resources, then empty namespace, then cluster
	if !strings.Contains(string(content), "resources=[Deployment/nginx@production] namespace= cluster=prod-cluster") {
		t.Errorf("file-based text line not in unified layout, got:\n%s", string(content))
	}
}

func TestLogUnknownFormatFallsBackToText(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	cfg := &config.Config{
		Audit: config.AuditConfig{
			Enabled: true,
			Path:    logPath,
			Format:  "yaml", // unrecognized → must fall back to text
		},
	}

	logger := New(cfg)
	result := &checker.CheckResult{
		Operation: "delete",
		Resources: []string{"pod/nginx"},
		Namespace: "default",
		Cluster:   "test-cluster",
	}

	if err := logger.Log(result, []string{"delete", "pod", "nginx"}, true, true); err != nil {
		t.Fatalf("Log() returned error: %v", err)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	line := strings.TrimSpace(string(content))
	// Must be the text line, not JSON
	if strings.HasPrefix(line, "{") {
		t.Errorf("unknown format should produce text, got JSON: %s", line)
	}
	if !strings.Contains(line, "operation=delete resources=[pod/nginx]") {
		t.Errorf("unknown format did not produce text layout, got: %s", line)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/audit -v -run 'TestLogJSONFormat|TestLogResourcesJSONFormat|TestLogResourcesTextUnified|TestLogUnknownFormatFallsBackToText'`
Expected: FAIL — `TestLogJSONFormat`/`TestLogResourcesJSONFormat` fail because output is still text (JSON unmarshal errors), and `TestLogResourcesTextUnified` fails because the file-based line still uses the old `cluster=...resources=...` order with no `namespace=`. (`TestLogUnknownFormatFallsBackToText` may already pass against the current text-only code — that is fine; it pins the fallback after the refactor.)

- [ ] **Step 3: Add `writeEntry` and refactor both methods**

In `internal/audit/audit.go`, add the shared writer (place it just below the formatters from Task 2):

```go
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
```

Replace the entire body of `Log` with:

```go
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
```

Replace the entire body of `LogResources` with:

```go
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
```

- [ ] **Step 4: Run the audit tests**

Run: `go test ./internal/audit -v`
Expected: ALL PASS — new JSON/unified tests pass; existing text tests still pass (CLI text is byte-for-byte identical; file-based substring assertions survive the reorder).

- [ ] **Step 5: Run the full suite and vet**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: clean; all packages `ok`. `gofmt -l internal/audit/ internal/config/` prints nothing.

- [ ] **Step 6: Manual smoke test**

```bash
go build -o safekubectl .
SAFEKUBECTL_CONFIG=/dev/null   # not used; shown for reference only
```

Create a temp config and exercise JSON output without touching a real cluster (answer `n` so kubectl never runs):

```bash
TMPCFG="$(mktemp -d)/config.yaml"
cat > "$TMPCFG" <<'EOF'
mode: confirm
dangerousOperations: [delete]
audit:
  enabled: true
  path: /tmp/safekubectl-smoke-audit.log
  format: json
EOF
rm -f /tmp/safekubectl-smoke-audit.log
echo "n" | SAFEKUBECTL_CONFIG="$TMPCFG" ./safekubectl delete secret cert-a cert-b -n istio-system
cat /tmp/safekubectl-smoke-audit.log
```

Expected: the audit file contains one JSON Lines entry (DENIED, since you answered `n`) with keys `timestamp,status,operation,resources,namespace,cluster,confirmed,command` and `"resources":["secret/cert-a","secret/cert-b"]`. Confirm it parses: `jq . /tmp/safekubectl-smoke-audit.log` (if `jq` is available). Then clean up:

```bash
rm -f safekubectl /tmp/safekubectl-smoke-audit.log
```

- [ ] **Step 7: Commit (ask the user first — see commit policy)**

```bash
git add internal/audit/audit.go internal/audit/audit_test.go
git commit -m "feat(audit): unify entry writing and support json output format"
```
