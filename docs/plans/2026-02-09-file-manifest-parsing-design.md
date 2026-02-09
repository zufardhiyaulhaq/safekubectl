# File Manifest Parsing Design

## Problem

When using `kubectl apply -f manifest.yaml`, safekubectl doesn't parse the YAML file to extract the actual namespace from the manifest metadata. It just uses the current context's namespace or `-n` flag, leading to incorrect safety checks.

Example: Applying a file targeting `istio-system` while current namespace is `default` shows the wrong namespace in warnings.

## Solution

Parse `-f` file inputs to extract resource metadata (kind, name, namespace) and use this for accurate safety checks.

## Scope

- **Supported formats:** YAML (including multi-document) and JSON
- **Supported sources:** Local files, directories, URLs
- **Not supported (for now):** stdin (`-f -`)

## Design

### New Package: `internal/manifest`

```
internal/
├── manifest/
│   ├── manifest.go      # Core types and parsing logic
│   ├── yaml.go          # YAML/multi-doc parsing
│   ├── json.go          # JSON parsing
│   └── fetcher.go       # URL fetching with confirmation
```

### Key Types

```go
// Resource represents a single parsed Kubernetes resource
type Resource struct {
    APIVersion string
    Kind       string
    Name       string
    Namespace  string  // empty if not specified in manifest
}

// ParseResult contains all resources from a -f source
type ParseResult struct {
    Resources []Resource
    Source    string  // file path or URL for display
}
```

### Main Function

```go
func Parse(filePath string, recursive bool, confirmFetch func(url string) bool) ([]ParseResult, error)
```

### Parsing Flow

1. **Detect source type:**
   - URL (starts with `http://` or `https://`) → call `confirmFetch()`, then fetch and parse
   - Directory → walk files (recursive only if `-R` flag present), parse each `.yaml`, `.yml`, `.json`
   - File → parse directly based on extension

2. **For each file:**
   - YAML: split by `---` delimiter, parse each document
   - JSON: parse as single resource (or array if it's a List kind)
   - Extract only: `apiVersion`, `kind`, `metadata.name`, `metadata.namespace`

3. **Return all resources grouped by source file**

### Error Handling

- File not found → return error, block execution
- Permission denied → return error, block execution
- Malformed YAML/JSON → return error with file path and line number if possible
- URL fetch fails → return error, block execution
- User declines URL fetch → return error indicating user cancelled

### Changes to `parser.KubectlCommand`

```go
type KubectlCommand struct {
    Operation   string
    Resource    string
    Name        string
    Namespace   string   // from -n flag (used as fallback)
    Context     string
    Args        []string
    FileInputs  []string // NEW: paths/URLs from -f flags
    Recursive   bool     // NEW: -R flag present
}
```

### Changes to `main.go` Runner

After parsing, if `cmd.FileInputs` is not empty:

1. Call `manifest.Parse()` for each file input
2. For resources without namespace in manifest:
   - Use `cmd.Namespace` if `-n` flag was provided
   - Else get current context's default namespace via `kubectl config view --minify -o jsonpath='{.contexts[0].context.namespace}'`
   - Else fall back to `default`
3. Pass the enriched resource list to the checker

### Changes to `checker.Check()`

New method to check multiple resources:

```go
func (c *Checker) CheckResources(resources []manifest.Resource, cluster string) *CheckResult
```

The `CheckResult.Reasons` will list each dangerous resource separately.

### Warning Display

**For file-based commands:**

```
⚠️  DANGEROUS OPERATION DETECTED

Command: kubectl apply -f ./manifests/

Resources affected:
  • Deployment/nginx in namespace istio-system
  • Service/nginx-svc in namespace istio-system
  • ConfigMap/config in namespace default

Cluster: production-cluster

Reasons:
  - Operation 'apply' on protected namespace 'istio-system'
  - Cluster 'production-cluster' is protected

Do you want to proceed? [y/N]:
```

**For URL sources (pre-fetch warning):**

```
⚠️  REMOTE MANIFEST WARNING

You are about to fetch a manifest from:
  https://raw.githubusercontent.com/example/manifest.yaml

Fetching remote manifests can be risky. Continue? [y/N]:
```

## Decisions Made

| Decision | Choice | Rationale |
|----------|--------|-----------|
| File formats | YAML + JSON | Most common manifest formats |
| Multi-resource handling | Treat as multiple commands | Clearest visibility of what's being applied |
| Missing namespace | Use `-n` flag, else context default | Match kubectl's actual behavior |
| Parse errors | Block execution | If we can't parse, kubectl will likely fail too |
| URL support | Fetch with confirmation | Support the feature but warn about risks |
| Directory support | Recursive, respect `-R` flag | Match kubectl's exact behavior |
