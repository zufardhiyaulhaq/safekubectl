# File Manifest Parsing Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Parse `-f` file inputs to extract resource metadata (kind, name, namespace) for accurate safety checks when using `kubectl apply -f`.

**Architecture:** Add new `internal/manifest` package for YAML/JSON parsing. Extend parser to capture `-f` flags and `-R` flag. Modify checker to evaluate resources from manifests. Update prompt display to show affected resources.

**Tech Stack:** Go 1.24, gopkg.in/yaml.v3 (already in go.mod), net/http for URL fetching

---

### Task 1: Create manifest package with core types

**Files:**
- Create: `internal/manifest/manifest.go`
- Create: `internal/manifest/manifest_test.go`

**Step 1: Write the failing test for Resource type**

```go
// internal/manifest/manifest_test.go
package manifest

import "testing"

func TestResourceString(t *testing.T) {
	r := Resource{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Name:       "nginx",
		Namespace:  "istio-system",
	}

	expected := "Deployment/nginx"
	if r.String() != expected {
		t.Errorf("String() = %q, expected %q", r.String(), expected)
	}
}

func TestResourceStringNoName(t *testing.T) {
	r := Resource{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Name:       "",
		Namespace:  "default",
	}

	expected := "ConfigMap"
	if r.String() != expected {
		t.Errorf("String() = %q, expected %q", r.String(), expected)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/manifest -v -run TestResource`
Expected: FAIL with "package not found" or similar

**Step 3: Write minimal implementation**

```go
// internal/manifest/manifest.go
package manifest

// Resource represents a single parsed Kubernetes resource
type Resource struct {
	APIVersion string
	Kind       string
	Name       string
	Namespace  string // empty if not specified in manifest
	Source     string // file path or URL for display
}

// String returns a display string like "Deployment/nginx"
func (r Resource) String() string {
	if r.Name == "" {
		return r.Kind
	}
	return r.Kind + "/" + r.Name
}

// ParseResult contains all resources from a -f source
type ParseResult struct {
	Resources []Resource
	Source    string // file path or URL for display
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/manifest -v -run TestResource`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/manifest/manifest.go internal/manifest/manifest_test.go
git commit -m "feat(manifest): add core types for resource parsing"
```

---

### Task 2: Implement YAML parsing for single documents

**Files:**
- Create: `internal/manifest/yaml.go`
- Modify: `internal/manifest/manifest_test.go`

**Step 1: Write the failing test for single YAML document**

```go
// Add to internal/manifest/manifest_test.go

func TestParseYAMLSingleDocument(t *testing.T) {
	content := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  namespace: istio-system
spec:
  replicas: 1`

	resources, err := ParseYAML([]byte(content), "test.yaml")
	if err != nil {
		t.Fatalf("ParseYAML() error = %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("Expected 1 resource, got %d", len(resources))
	}

	r := resources[0]
	if r.APIVersion != "apps/v1" {
		t.Errorf("APIVersion = %q, expected %q", r.APIVersion, "apps/v1")
	}
	if r.Kind != "Deployment" {
		t.Errorf("Kind = %q, expected %q", r.Kind, "Deployment")
	}
	if r.Name != "nginx" {
		t.Errorf("Name = %q, expected %q", r.Name, "nginx")
	}
	if r.Namespace != "istio-system" {
		t.Errorf("Namespace = %q, expected %q", r.Namespace, "istio-system")
	}
	if r.Source != "test.yaml" {
		t.Errorf("Source = %q, expected %q", r.Source, "test.yaml")
	}
}

func TestParseYAMLNoNamespace(t *testing.T) {
	content := `apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
data:
  key: value`

	resources, err := ParseYAML([]byte(content), "configmap.yaml")
	if err != nil {
		t.Fatalf("ParseYAML() error = %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("Expected 1 resource, got %d", len(resources))
	}

	if resources[0].Namespace != "" {
		t.Errorf("Namespace = %q, expected empty", resources[0].Namespace)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/manifest -v -run TestParseYAML`
Expected: FAIL with "ParseYAML not defined"

**Step 3: Write minimal implementation**

```go
// internal/manifest/yaml.go
package manifest

import (
	"bytes"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// kubeResource represents the minimal structure we need from a Kubernetes manifest
type kubeResource struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
}

// ParseYAML parses YAML content and extracts Kubernetes resources
// Supports multi-document YAML (separated by ---)
func ParseYAML(content []byte, source string) ([]Resource, error) {
	var resources []Resource

	decoder := yaml.NewDecoder(bytes.NewReader(content))
	for {
		var doc kubeResource
		err := decoder.Decode(&doc)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to parse YAML from %s: %w", source, err)
		}

		// Skip empty documents (can happen with --- separators)
		if doc.Kind == "" {
			continue
		}

		resources = append(resources, Resource{
			APIVersion: doc.APIVersion,
			Kind:       doc.Kind,
			Name:       doc.Metadata.Name,
			Namespace:  doc.Metadata.Namespace,
			Source:     source,
		})
	}

	return resources, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/manifest -v -run TestParseYAML`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/manifest/yaml.go internal/manifest/manifest_test.go
git commit -m "feat(manifest): add YAML parsing for single documents"
```

---

### Task 3: Implement YAML parsing for multi-document files

**Files:**
- Modify: `internal/manifest/manifest_test.go`

**Step 1: Write the failing test for multi-document YAML**

```go
// Add to internal/manifest/manifest_test.go

func TestParseYAMLMultiDocument(t *testing.T) {
	content := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  namespace: istio-system
---
apiVersion: v1
kind: Service
metadata:
  name: nginx-svc
  namespace: istio-system
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: config
  namespace: default`

	resources, err := ParseYAML([]byte(content), "multi.yaml")
	if err != nil {
		t.Fatalf("ParseYAML() error = %v", err)
	}

	if len(resources) != 3 {
		t.Fatalf("Expected 3 resources, got %d", len(resources))
	}

	expected := []struct {
		kind      string
		name      string
		namespace string
	}{
		{"Deployment", "nginx", "istio-system"},
		{"Service", "nginx-svc", "istio-system"},
		{"ConfigMap", "config", "default"},
	}

	for i, exp := range expected {
		if resources[i].Kind != exp.kind {
			t.Errorf("resources[%d].Kind = %q, expected %q", i, resources[i].Kind, exp.kind)
		}
		if resources[i].Name != exp.name {
			t.Errorf("resources[%d].Name = %q, expected %q", i, resources[i].Name, exp.name)
		}
		if resources[i].Namespace != exp.namespace {
			t.Errorf("resources[%d].Namespace = %q, expected %q", i, resources[i].Namespace, exp.namespace)
		}
	}
}

func TestParseYAMLWithEmptyDocuments(t *testing.T) {
	content := `---
apiVersion: v1
kind: Pod
metadata:
  name: nginx
---
---
`

	resources, err := ParseYAML([]byte(content), "empty-docs.yaml")
	if err != nil {
		t.Fatalf("ParseYAML() error = %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("Expected 1 resource (skipping empty docs), got %d", len(resources))
	}
}
```

**Step 2: Run test to verify it passes (already implemented in Task 2)**

Run: `go test ./internal/manifest -v -run TestParseYAMLMulti`
Expected: PASS (multi-doc support already in ParseYAML)

**Step 3: Commit**

```bash
git add internal/manifest/manifest_test.go
git commit -m "test(manifest): add multi-document YAML tests"
```

---

### Task 4: Implement JSON parsing

**Files:**
- Create: `internal/manifest/json.go`
- Modify: `internal/manifest/manifest_test.go`

**Step 1: Write the failing test for JSON parsing**

```go
// Add to internal/manifest/manifest_test.go

func TestParseJSONSingleResource(t *testing.T) {
	content := `{
  "apiVersion": "apps/v1",
  "kind": "Deployment",
  "metadata": {
    "name": "nginx",
    "namespace": "production"
  }
}`

	resources, err := ParseJSON([]byte(content), "deploy.json")
	if err != nil {
		t.Fatalf("ParseJSON() error = %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("Expected 1 resource, got %d", len(resources))
	}

	r := resources[0]
	if r.Kind != "Deployment" {
		t.Errorf("Kind = %q, expected %q", r.Kind, "Deployment")
	}
	if r.Name != "nginx" {
		t.Errorf("Name = %q, expected %q", r.Name, "nginx")
	}
	if r.Namespace != "production" {
		t.Errorf("Namespace = %q, expected %q", r.Namespace, "production")
	}
}

func TestParseJSONList(t *testing.T) {
	content := `{
  "apiVersion": "v1",
  "kind": "List",
  "items": [
    {
      "apiVersion": "v1",
      "kind": "Pod",
      "metadata": {"name": "pod1", "namespace": "ns1"}
    },
    {
      "apiVersion": "v1",
      "kind": "Pod",
      "metadata": {"name": "pod2", "namespace": "ns2"}
    }
  ]
}`

	resources, err := ParseJSON([]byte(content), "list.json")
	if err != nil {
		t.Fatalf("ParseJSON() error = %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("Expected 2 resources, got %d", len(resources))
	}

	if resources[0].Name != "pod1" || resources[1].Name != "pod2" {
		t.Errorf("Unexpected resource names: %v, %v", resources[0].Name, resources[1].Name)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/manifest -v -run TestParseJSON`
Expected: FAIL with "ParseJSON not defined"

**Step 3: Write minimal implementation**

```go
// internal/manifest/json.go
package manifest

import (
	"encoding/json"
	"fmt"
)

// kubeResourceJSON mirrors kubeResource for JSON unmarshaling
type kubeResourceJSON struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Items []kubeResourceJSON `json:"items,omitempty"`
}

// ParseJSON parses JSON content and extracts Kubernetes resources
// Supports both single resources and List kinds
func ParseJSON(content []byte, source string) ([]Resource, error) {
	var doc kubeResourceJSON
	if err := json.Unmarshal(content, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse JSON from %s: %w", source, err)
	}

	var resources []Resource

	// Handle List kind
	if doc.Kind == "List" && len(doc.Items) > 0 {
		for _, item := range doc.Items {
			if item.Kind == "" {
				continue
			}
			resources = append(resources, Resource{
				APIVersion: item.APIVersion,
				Kind:       item.Kind,
				Name:       item.Metadata.Name,
				Namespace:  item.Metadata.Namespace,
				Source:     source,
			})
		}
		return resources, nil
	}

	// Single resource
	if doc.Kind != "" {
		resources = append(resources, Resource{
			APIVersion: doc.APIVersion,
			Kind:       doc.Kind,
			Name:       doc.Metadata.Name,
			Namespace:  doc.Metadata.Namespace,
			Source:     source,
		})
	}

	return resources, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/manifest -v -run TestParseJSON`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/manifest/json.go internal/manifest/manifest_test.go
git commit -m "feat(manifest): add JSON parsing with List support"
```

---

### Task 5: Implement file type detection and unified Parse function

**Files:**
- Modify: `internal/manifest/manifest.go`
- Modify: `internal/manifest/manifest_test.go`

**Step 1: Write the failing test for ParseFile**

```go
// Add to internal/manifest/manifest_test.go

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFileYAML(t *testing.T) {
	// Create temp file
	dir := t.TempDir()
	path := filepath.Join(dir, "deploy.yaml")
	content := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  namespace: test`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	resources, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("Expected 1 resource, got %d", len(resources))
	}
	if resources[0].Kind != "Deployment" {
		t.Errorf("Kind = %q, expected Deployment", resources[0].Kind)
	}
}

func TestParseFileJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deploy.json")
	content := `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"nginx"}}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	resources, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("Expected 1 resource, got %d", len(resources))
	}
	if resources[0].Kind != "Pod" {
		t.Errorf("Kind = %q, expected Pod", resources[0].Kind)
	}
}

func TestParseFileNotFound(t *testing.T) {
	_, err := ParseFile("/nonexistent/file.yaml")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestParseFileUnsupportedExtension(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := ParseFile(path)
	if err == nil {
		t.Error("Expected error for unsupported extension")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/manifest -v -run TestParseFile`
Expected: FAIL with "ParseFile not defined"

**Step 3: Write minimal implementation**

```go
// Add to internal/manifest/manifest.go

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ParseFile parses a file based on its extension
func ParseFile(path string) ([]Resource, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		return ParseYAML(content, path)
	case ".json":
		return ParseJSON(content, path)
	default:
		return nil, fmt.Errorf("unsupported file extension %q for %s", ext, path)
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/manifest -v -run TestParseFile`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/manifest/manifest.go internal/manifest/manifest_test.go
git commit -m "feat(manifest): add ParseFile with extension detection"
```

---

### Task 6: Implement directory parsing

**Files:**
- Modify: `internal/manifest/manifest.go`
- Modify: `internal/manifest/manifest_test.go`

**Step 1: Write the failing test for ParseDirectory**

```go
// Add to internal/manifest/manifest_test.go

func TestParseDirectoryNonRecursive(t *testing.T) {
	dir := t.TempDir()

	// Create files in root
	os.WriteFile(filepath.Join(dir, "deploy.yaml"), []byte(`apiVersion: v1
kind: Pod
metadata:
  name: pod1`), 0644)
	os.WriteFile(filepath.Join(dir, "svc.json"), []byte(`{"apiVersion":"v1","kind":"Service","metadata":{"name":"svc1"}}`), 0644)

	// Create subdir with file (should be ignored in non-recursive)
	subdir := filepath.Join(dir, "subdir")
	os.Mkdir(subdir, 0755)
	os.WriteFile(filepath.Join(subdir, "nested.yaml"), []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: cm1`), 0644)

	resources, err := ParseDirectory(dir, false)
	if err != nil {
		t.Fatalf("ParseDirectory() error = %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("Expected 2 resources (non-recursive), got %d", len(resources))
	}
}

func TestParseDirectoryRecursive(t *testing.T) {
	dir := t.TempDir()

	// Create files in root
	os.WriteFile(filepath.Join(dir, "deploy.yaml"), []byte(`apiVersion: v1
kind: Pod
metadata:
  name: pod1`), 0644)

	// Create subdir with file
	subdir := filepath.Join(dir, "subdir")
	os.Mkdir(subdir, 0755)
	os.WriteFile(filepath.Join(subdir, "nested.yaml"), []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: cm1`), 0644)

	resources, err := ParseDirectory(dir, true)
	if err != nil {
		t.Fatalf("ParseDirectory() error = %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("Expected 2 resources (recursive), got %d", len(resources))
	}
}

func TestParseDirectoryNotExists(t *testing.T) {
	_, err := ParseDirectory("/nonexistent/dir", false)
	if err == nil {
		t.Error("Expected error for nonexistent directory")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/manifest -v -run TestParseDirectory`
Expected: FAIL with "ParseDirectory not defined"

**Step 3: Write minimal implementation**

```go
// Add to internal/manifest/manifest.go

import (
	"io/fs"
	"path/filepath"
)

// isSupportedFile returns true if the file has a supported extension
func isSupportedFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml" || ext == ".json"
}

// ParseDirectory parses all YAML/JSON files in a directory
func ParseDirectory(dir string, recursive bool) ([]Resource, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to access directory %s: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}

	var resources []Resource

	if recursive {
		err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !isSupportedFile(path) {
				return nil
			}
			res, err := ParseFile(path)
			if err != nil {
				return err
			}
			resources = append(resources, res...)
			return nil
		})
	} else {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, fmt.Errorf("failed to read directory %s: %w", dir, err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			if !isSupportedFile(path) {
				continue
			}
			res, err := ParseFile(path)
			if err != nil {
				return nil, err
			}
			resources = append(resources, res...)
		}
	}

	if err != nil {
		return nil, err
	}

	return resources, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/manifest -v -run TestParseDirectory`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/manifest/manifest.go internal/manifest/manifest_test.go
git commit -m "feat(manifest): add directory parsing with recursive support"
```

---

### Task 7: Implement URL fetching with confirmation

**Files:**
- Create: `internal/manifest/fetcher.go`
- Modify: `internal/manifest/manifest_test.go`

**Step 1: Write the failing test for URL detection and fetching**

```go
// Add to internal/manifest/manifest_test.go

func TestIsURL(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"https://example.com/manifest.yaml", true},
		{"http://example.com/manifest.yaml", true},
		{"./local/file.yaml", false},
		{"/absolute/path.yaml", false},
		{"file.yaml", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := IsURL(tt.input); got != tt.expected {
				t.Errorf("IsURL(%q) = %v, expected %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFetchURLUserDeclines(t *testing.T) {
	confirmFunc := func(url string) bool {
		return false // User declines
	}

	_, err := FetchURL("https://example.com/manifest.yaml", confirmFunc)
	if err == nil {
		t.Error("Expected error when user declines")
	}
	if !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("Expected 'cancelled' in error, got: %v", err)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/manifest -v -run "TestIsURL|TestFetchURL"`
Expected: FAIL with "IsURL/FetchURL not defined"

**Step 3: Write minimal implementation**

```go
// internal/manifest/fetcher.go
package manifest

import (
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"
)

// IsURL returns true if the path looks like a URL
func IsURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// FetchURL fetches content from a URL after user confirmation
// confirmFunc is called with the URL; if it returns false, fetch is cancelled
func FetchURL(url string, confirmFunc func(url string) bool) ([]byte, error) {
	if !confirmFunc(url) {
		return nil, fmt.Errorf("fetch cancelled by user for URL: %s", url)
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch URL %s: status %d", url, resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response from %s: %w", url, err)
	}

	return content, nil
}

// ParseURL fetches and parses a manifest from a URL
func ParseURL(url string, confirmFunc func(url string) bool) ([]Resource, error) {
	content, err := FetchURL(url, confirmFunc)
	if err != nil {
		return nil, err
	}

	// Determine file type from URL path
	ext := strings.ToLower(path.Ext(url))
	switch ext {
	case ".json":
		return ParseJSON(content, url)
	case ".yaml", ".yml":
		return ParseYAML(content, url)
	default:
		// Default to YAML for unknown extensions (common for raw GitHub URLs)
		return ParseYAML(content, url)
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/manifest -v -run "TestIsURL|TestFetchURL"`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/manifest/fetcher.go internal/manifest/manifest_test.go
git commit -m "feat(manifest): add URL fetching with user confirmation"
```

---

### Task 8: Implement unified Parse function

**Files:**
- Modify: `internal/manifest/manifest.go`
- Modify: `internal/manifest/manifest_test.go`

**Step 1: Write the failing test for unified Parse function**

```go
// Add to internal/manifest/manifest_test.go

func TestParseLocalFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deploy.yaml")
	content := `apiVersion: v1
kind: Pod
metadata:
  name: test`
	os.WriteFile(path, []byte(content), 0644)

	confirmFunc := func(url string) bool { return true }
	resources, err := Parse(path, false, confirmFunc)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("Expected 1 resource, got %d", len(resources))
	}
}

func TestParseLocalDirectory(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.yaml"), []byte(`apiVersion: v1
kind: Pod
metadata:
  name: a`), 0644)
	os.WriteFile(filepath.Join(dir, "b.yaml"), []byte(`apiVersion: v1
kind: Pod
metadata:
  name: b`), 0644)

	confirmFunc := func(url string) bool { return true }
	resources, err := Parse(dir, false, confirmFunc)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("Expected 2 resources, got %d", len(resources))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/manifest -v -run "TestParseLocal"`
Expected: FAIL with "Parse not defined" or wrong signature

**Step 3: Write minimal implementation**

```go
// Add to internal/manifest/manifest.go

// Parse parses a file path, directory, or URL and returns all resources
// - For URLs: calls confirmFunc before fetching
// - For directories: respects recursive flag
// - For files: parses based on extension
func Parse(source string, recursive bool, confirmFunc func(url string) bool) ([]Resource, error) {
	// Handle URLs
	if IsURL(source) {
		return ParseURL(source, confirmFunc)
	}

	// Check if source exists
	info, err := os.Stat(source)
	if err != nil {
		return nil, fmt.Errorf("failed to access %s: %w", source, err)
	}

	// Handle directories
	if info.IsDir() {
		return ParseDirectory(source, recursive)
	}

	// Handle files
	return ParseFile(source)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/manifest -v -run "TestParseLocal"`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/manifest/manifest.go internal/manifest/manifest_test.go
git commit -m "feat(manifest): add unified Parse function"
```

---

### Task 9: Extend parser to capture -f flags and -R flag

**Files:**
- Modify: `internal/parser/parser.go`
- Modify: `internal/parser/parser_test.go`

**Step 1: Write the failing test for file inputs parsing**

```go
// Add to internal/parser/parser_test.go

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
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/parser -v -run TestParseFileInputs`
Expected: FAIL with "FileInputs/Recursive field not defined"

**Step 3: Write minimal implementation**

Update `internal/parser/parser.go`:

```go
// KubectlCommand represents a parsed kubectl command
type KubectlCommand struct {
	Operation  string   // e.g., delete, apply, get
	Resource   string   // e.g., pod, deployment, pod/nginx
	Name       string   // e.g., nginx (if separate from resource)
	Namespace  string   // from -n or --namespace flag
	Context    string   // from --context flag
	Args       []string // original arguments
	FileInputs []string // paths/URLs from -f/--filename flags
	Recursive  bool     // -R/--recursive flag present
}
```

Add to the parsing logic in Parse():

```go
// Handle file input flags
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

// Handle recursive flag
if arg == "-R" || arg == "--recursive" {
	cmd.Recursive = true
	i++
	continue
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/parser -v -run TestParseFileInputs`
Expected: PASS

**Step 5: Run all parser tests**

Run: `go test ./internal/parser -v`
Expected: All PASS

**Step 6: Commit**

```bash
git add internal/parser/parser.go internal/parser/parser_test.go
git commit -m "feat(parser): capture -f flags and -R recursive flag"
```

---

### Task 10: Add CheckResources method to checker

**Files:**
- Modify: `internal/checker/checker.go`
- Modify: `internal/checker/checker_test.go`

**Step 1: Write the failing test for CheckResources**

```go
// Add to internal/checker/checker_test.go

import (
	"github.com/zufardhiyaulhaq/safekubectl/internal/manifest"
)

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

	// Should have at least 2 reasons: dangerous op + protected namespace
	if len(result.Reasons) < 2 {
		t.Errorf("Expected at least 2 reasons, got %d: %v", len(result.Reasons), result.Reasons)
	}

	// Should require confirmation due to protected namespace
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

	// "get" is not dangerous
	result := chk.CheckResources("get", resources, "dev-cluster")

	if result.IsDangerous {
		t.Error("Expected IsDangerous=false for get operation")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/checker -v -run TestCheckResources`
Expected: FAIL with "CheckResources not defined"

**Step 3: Write minimal implementation**

```go
// Add to internal/checker/checker.go

import (
	"github.com/zufardhiyaulhaq/safekubectl/internal/manifest"
)

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
			ns = "default" // Will be resolved later, but check default for now
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
		for ns := range protectedNamespaces {
			if c.config.IsProtectedNamespace(ns) {
				result.RequiresConfirmation = true
				break
			}
		}
		if c.config.IsProtectedCluster(cluster) {
			result.RequiresConfirmation = true
		}
	}

	return result
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/checker -v -run TestCheckResources`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/checker/checker.go internal/checker/checker_test.go
git commit -m "feat(checker): add CheckResources for manifest-based commands"
```

---

### Task 11: Add display functions for resource-based warnings

**Files:**
- Modify: `internal/prompt/prompt.go`
- Modify: `internal/prompt/prompt_test.go`

**Step 1: Write the failing test for resource warning display**

```go
// Add to internal/prompt/prompt_test.go

import (
	"bytes"
	"strings"
	"testing"

	"github.com/zufardhiyaulhaq/safekubectl/internal/checker"
	"github.com/zufardhiyaulhaq/safekubectl/internal/manifest"
)

func TestDisplayResourceWarning(t *testing.T) {
	result := &checker.ResourceCheckResult{
		IsDangerous:          true,
		RequiresConfirmation: true,
		Operation:            "apply",
		Cluster:              "prod-cluster",
		Resources: []manifest.Resource{
			{Kind: "Deployment", Name: "nginx", Namespace: "istio-system", Source: "deploy.yaml"},
			{Kind: "Service", Name: "nginx-svc", Namespace: "default", Source: "deploy.yaml"},
		},
		Reasons: []string{"dangerous operation: apply", "protected namespace: istio-system"},
	}

	var buf bytes.Buffer
	DisplayResourceWarningTo(&buf, result, []string{"apply", "-f", "deploy.yaml"})

	output := buf.String()

	if !strings.Contains(output, "DANGEROUS OPERATION") {
		t.Error("Expected warning header")
	}
	if !strings.Contains(output, "Deployment/nginx") {
		t.Error("Expected Deployment/nginx in output")
	}
	if !strings.Contains(output, "istio-system") {
		t.Error("Expected istio-system namespace in output")
	}
	if !strings.Contains(output, "Service/nginx-svc") {
		t.Error("Expected Service/nginx-svc in output")
	}
	if !strings.Contains(output, "prod-cluster") {
		t.Error("Expected cluster name in output")
	}
}

func TestDisplayURLWarning(t *testing.T) {
	var buf bytes.Buffer
	DisplayURLWarningTo(&buf, "https://example.com/manifest.yaml")

	output := buf.String()

	if !strings.Contains(output, "REMOTE MANIFEST") {
		t.Error("Expected remote manifest warning")
	}
	if !strings.Contains(output, "https://example.com/manifest.yaml") {
		t.Error("Expected URL in output")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/prompt -v -run "TestDisplayResource|TestDisplayURL"`
Expected: FAIL with functions not defined

**Step 3: Write minimal implementation**

```go
// Add to internal/prompt/prompt.go

import (
	"github.com/zufardhiyaulhaq/safekubectl/internal/manifest"
)

// DisplayResourceWarning shows the danger warning for file-based commands
func DisplayResourceWarning(result *checker.ResourceCheckResult, args []string) {
	DisplayResourceWarningTo(os.Stdout, result, args)
}

// DisplayResourceWarningTo writes the resource warning to the specified writer
func DisplayResourceWarningTo(w io.Writer, result *checker.ResourceCheckResult, args []string) {
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s%s  DANGEROUS OPERATION DETECTED%s\n", colorYellow, warningIcon(), colorReset)
	fmt.Fprintf(w, "├── Operation: %s%s%s\n", colorRed, result.Operation, colorReset)
	fmt.Fprintf(w, "├── Cluster:   %s\n", result.Cluster)
	fmt.Fprintf(w, "├── Command:   kubectl %s\n", strings.Join(args, " "))
	fmt.Fprintln(w, "│")
	fmt.Fprintln(w, "├── Resources affected:")

	for i, r := range result.Resources {
		prefix := "│   ├──"
		if i == len(result.Resources)-1 {
			prefix = "│   └──"
		}
		ns := r.Namespace
		if ns == "" {
			ns = "(unspecified)"
		}
		fmt.Fprintf(w, "%s %s in namespace %s\n", prefix, r.String(), ns)
	}

	if len(result.Reasons) > 0 {
		fmt.Fprintln(w, "│")
		fmt.Fprintln(w, "└── Reasons:")
		for i, reason := range result.Reasons {
			prefix := "    ├──"
			if i == len(result.Reasons)-1 {
				prefix = "    └──"
			}
			fmt.Fprintf(w, "%s %s\n", prefix, reason)
		}
	}

	fmt.Fprintln(w)
}

// DisplayURLWarning shows the warning before fetching a remote manifest
func DisplayURLWarning(url string) {
	DisplayURLWarningTo(os.Stdout, url)
}

// DisplayURLWarningTo writes the URL warning to the specified writer
func DisplayURLWarningTo(w io.Writer, url string) {
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s%s  REMOTE MANIFEST WARNING%s\n", colorYellow, warningIcon(), colorReset)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "You are about to fetch a manifest from:")
	fmt.Fprintf(w, "  %s\n", url)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Fetching remote manifests can be risky.")
	fmt.Fprintln(w)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/prompt -v -run "TestDisplayResource|TestDisplayURL"`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/prompt/prompt.go internal/prompt/prompt_test.go
git commit -m "feat(prompt): add display functions for resource-based warnings"
```

---

### Task 12: Integrate manifest parsing into main.go

**Files:**
- Modify: `main.go`
- Modify: `main_test.go`

**Step 1: Write the failing test for file-based apply integration**

```go
// Add to main_test.go

func TestRunWithFileInput(t *testing.T) {
	// Create temp manifest file
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "deploy.yaml")
	content := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  namespace: istio-system`
	os.WriteFile(manifestPath, []byte(content), 0644)

	cfg := &config.Config{
		Mode:                config.ModeConfirm,
		DangerousOperations: []string{"apply"},
		ProtectedNamespaces: []string{"istio-system"},
		ProtectedClusters:   []string{},
	}

	var stdout, stderr bytes.Buffer
	stdin := strings.NewReader("n\n") // User declines

	runner := &Runner{
		stdin:          stdin,
		stdout:         &stdout,
		stderr:         &stderr,
		getCluster:     func() string { return "test-cluster" },
		getContextNamespace: func() string { return "default" },
		executeKubectl: func(args []string) error { return nil },
		loadConfig:     func() (*config.Config, error) { return cfg, nil },
	}

	err := runner.Run([]string{"apply", "-f", manifestPath})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	output := stdout.String()
	// Should show warning with correct namespace from manifest
	if !strings.Contains(output, "istio-system") {
		t.Errorf("Expected 'istio-system' in output, got: %s", output)
	}
	if !strings.Contains(output, "Deployment/nginx") {
		t.Errorf("Expected 'Deployment/nginx' in output, got: %s", output)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v -run TestRunWithFileInput`
Expected: FAIL (manifest parsing not integrated)

**Step 3: Write minimal implementation**

Update main.go Run() function:

```go
import (
	"github.com/zufardhiyaulhaq/safekubectl/internal/manifest"
)

// Runner encapsulates the main execution logic
type Runner struct {
	stdin               io.Reader
	stdout              io.Writer
	stderr              io.Writer
	getCluster          func() string
	getContextNamespace func() string  // NEW: get default namespace from context
	executeKubectl      func(args []string) error
	loadConfig          func() (*config.Config, error)
}

// Run executes the main logic
func (r *Runner) Run(args []string) error {
	// ... existing code up to cmd := parser.Parse(args) ...

	// Handle file-based commands
	if len(cmd.FileInputs) > 0 {
		return r.runWithFileInputs(cmd, cfg, cluster, args)
	}

	// ... existing code for non-file commands ...
}

// runWithFileInputs handles commands with -f flags
func (r *Runner) runWithFileInputs(cmd *parser.KubectlCommand, cfg *config.Config, cluster string, args []string) error {
	// Collect all resources from all file inputs
	var allResources []manifest.Resource

	confirmURL := func(url string) bool {
		prompt.DisplayURLWarningTo(r.stdout)
		return prompt.AskConfirmationFrom(r.stdin, r.stdout)
	}

	for _, fileInput := range cmd.FileInputs {
		resources, err := manifest.Parse(fileInput, cmd.Recursive, confirmURL)
		if err != nil {
			return fmt.Errorf("failed to parse %s: %w", fileInput, err)
		}
		allResources = append(allResources, resources...)
	}

	// Resolve empty namespaces
	fallbackNS := cmd.Namespace
	if fallbackNS == "" {
		fallbackNS = r.getContextNamespace()
	}
	if fallbackNS == "" {
		fallbackNS = "default"
	}

	for i := range allResources {
		if allResources[i].Namespace == "" {
			allResources[i].Namespace = fallbackNS
		}
	}

	// Check resources
	chk := checker.New(cfg)
	result := chk.CheckResources(cmd.Operation, allResources, cluster)

	// Initialize audit logger
	auditLogger := audit.New(cfg)

	// If not dangerous, execute directly
	if !result.IsDangerous {
		return r.executeKubectl(args)
	}

	// Display warning
	prompt.DisplayResourceWarningTo(r.stdout, result, args)

	// Handle confirmation
	confirmed := false
	if result.RequiresConfirmation {
		confirmed = prompt.AskConfirmationFrom(r.stdin, r.stdout)
		if !confirmed {
			prompt.DisplayAbortedTo(r.stdout)
			return nil
		}
	} else {
		prompt.DisplayProceedingTo(r.stdout)
		confirmed = true
	}

	// Execute kubectl
	return r.executeKubectl(args)
}
```

Also add helper to get context namespace:

```go
// getContextDefaultNamespace gets the default namespace from current context
func getContextDefaultNamespace() string {
	cmd := exec.Command("kubectl", "config", "view", "--minify", "-o", "jsonpath={.contexts[0].context.namespace}")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}
```

Update main():

```go
func main() {
	runner := &Runner{
		stdin:               os.Stdin,
		stdout:              os.Stdout,
		stderr:              os.Stderr,
		getCluster:          getCurrentCluster,
		getContextNamespace: getContextDefaultNamespace,
		executeKubectl:      executeKubectl,
		loadConfig:          config.Load,
	}
	// ...
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v -run TestRunWithFileInput`
Expected: PASS

**Step 5: Run all tests**

Run: `go test ./... -v`
Expected: All PASS

**Step 6: Commit**

```bash
git add main.go main_test.go
git commit -m "feat: integrate manifest parsing for -f file inputs"
```

---

### Task 13: Add integration tests for end-to-end scenarios

**Files:**
- Modify: `main_test.go`

**Step 1: Write integration tests**

```go
// Add to main_test.go

func TestIntegrationMultiDocYAML(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "multi.yaml")
	content := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  namespace: istio-system
---
apiVersion: v1
kind: Service
metadata:
  name: nginx-svc
  namespace: default`
	os.WriteFile(manifestPath, []byte(content), 0644)

	cfg := &config.Config{
		Mode:                config.ModeConfirm,
		DangerousOperations: []string{"apply"},
		ProtectedNamespaces: []string{"istio-system"},
		ProtectedClusters:   []string{},
	}

	var stdout bytes.Buffer
	stdin := strings.NewReader("n\n")

	runner := &Runner{
		stdin:               stdin,
		stdout:              &stdout,
		stderr:              &bytes.Buffer{},
		getCluster:          func() string { return "test" },
		getContextNamespace: func() string { return "default" },
		executeKubectl:      func(args []string) error { return nil },
		loadConfig:          func() (*config.Config, error) { return cfg, nil },
	}

	runner.Run([]string{"apply", "-f", manifestPath})

	output := stdout.String()
	// Both resources should be listed
	if !strings.Contains(output, "Deployment/nginx") {
		t.Error("Expected Deployment/nginx")
	}
	if !strings.Contains(output, "Service/nginx-svc") {
		t.Error("Expected Service/nginx-svc")
	}
	// Both namespaces should appear
	if !strings.Contains(output, "istio-system") {
		t.Error("Expected istio-system namespace")
	}
}

func TestIntegrationDirectoryRecursive(t *testing.T) {
	dir := t.TempDir()

	// Root level file
	os.WriteFile(filepath.Join(dir, "root.yaml"), []byte(`apiVersion: v1
kind: Pod
metadata:
  name: root-pod`), 0644)

	// Nested file
	subdir := filepath.Join(dir, "nested")
	os.Mkdir(subdir, 0755)
	os.WriteFile(filepath.Join(subdir, "nested.yaml"), []byte(`apiVersion: v1
kind: Pod
metadata:
  name: nested-pod`), 0644)

	cfg := &config.Config{
		Mode:                config.ModeConfirm,
		DangerousOperations: []string{"apply"},
		ProtectedNamespaces: []string{},
		ProtectedClusters:   []string{},
	}

	var stdout bytes.Buffer
	stdin := strings.NewReader("n\n")

	runner := &Runner{
		stdin:               stdin,
		stdout:              &stdout,
		stderr:              &bytes.Buffer{},
		getCluster:          func() string { return "test" },
		getContextNamespace: func() string { return "default" },
		executeKubectl:      func(args []string) error { return nil },
		loadConfig:          func() (*config.Config, error) { return cfg, nil },
	}

	// Without -R, should only get root-pod
	runner.Run([]string{"apply", "-f", dir})
	output := stdout.String()
	if !strings.Contains(output, "root-pod") {
		t.Error("Expected root-pod")
	}
	if strings.Contains(output, "nested-pod") {
		t.Error("Should not include nested-pod without -R")
	}

	// With -R, should get both
	stdout.Reset()
	stdin = strings.NewReader("n\n")
	runner.stdin = stdin
	runner.Run([]string{"apply", "-f", dir, "-R"})
	output = stdout.String()
	if !strings.Contains(output, "root-pod") {
		t.Error("Expected root-pod with -R")
	}
	if !strings.Contains(output, "nested-pod") {
		t.Error("Expected nested-pod with -R")
	}
}

func TestIntegrationFallbackNamespace(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "no-ns.yaml")
	// Manifest without namespace
	content := `apiVersion: v1
kind: Pod
metadata:
  name: test-pod`
	os.WriteFile(manifestPath, []byte(content), 0644)

	cfg := &config.Config{
		Mode:                config.ModeConfirm,
		DangerousOperations: []string{"apply"},
		ProtectedNamespaces: []string{"my-namespace"},
		ProtectedClusters:   []string{},
	}

	var stdout bytes.Buffer
	stdin := strings.NewReader("n\n")

	runner := &Runner{
		stdin:               stdin,
		stdout:              &stdout,
		stderr:              &bytes.Buffer{},
		getCluster:          func() string { return "test" },
		getContextNamespace: func() string { return "my-namespace" }, // Context default
		executeKubectl:      func(args []string) error { return nil },
		loadConfig:          func() (*config.Config, error) { return cfg, nil },
	}

	runner.Run([]string{"apply", "-f", manifestPath})

	output := stdout.String()
	// Should use context namespace and detect it's protected
	if !strings.Contains(output, "my-namespace") {
		t.Error("Expected my-namespace (from context)")
	}
	if !strings.Contains(output, "protected namespace") {
		t.Error("Expected protected namespace warning")
	}
}
```

**Step 2: Run integration tests**

Run: `go test -v -run TestIntegration`
Expected: All PASS

**Step 3: Commit**

```bash
git add main_test.go
git commit -m "test: add integration tests for manifest parsing scenarios"
```

---

### Task 14: Final verification and cleanup

**Step 1: Run all tests with coverage**

Run: `go test ./... -v -cover`
Expected: All PASS, coverage > 70%

**Step 2: Build and manual test**

```bash
go build -o safekubectl .
```

Create test manifest:
```bash
cat > /tmp/test-manifest.yaml << 'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  namespace: istio-system
---
apiVersion: v1
kind: Service
metadata:
  name: nginx-svc
EOF
```

Test:
```bash
./safekubectl apply -f /tmp/test-manifest.yaml
```

Expected: Warning shows both resources with correct namespaces

**Step 3: Commit final changes if any**

```bash
git add -A
git commit -m "chore: final cleanup and verification"
```

---

## Summary

| Task | Description | Estimated Steps |
|------|-------------|-----------------|
| 1 | Core types (Resource, ParseResult) | 5 |
| 2 | YAML parsing single document | 5 |
| 3 | YAML multi-document tests | 3 |
| 4 | JSON parsing with List support | 5 |
| 5 | ParseFile with extension detection | 5 |
| 6 | Directory parsing | 5 |
| 7 | URL fetching with confirmation | 5 |
| 8 | Unified Parse function | 5 |
| 9 | Parser: capture -f and -R flags | 6 |
| 10 | Checker: CheckResources method | 5 |
| 11 | Prompt: resource warning display | 5 |
| 12 | Main integration | 6 |
| 13 | Integration tests | 3 |
| 14 | Final verification | 3 |
