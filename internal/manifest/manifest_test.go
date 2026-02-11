package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
		t.Errorf("Unexpected names: %v, %v", resources[0].Name, resources[1].Name)
	}
}

func TestParseJSONEmptyList(t *testing.T) {
	// Bug: Empty list was returning "List" as a resource instead of empty
	content := `{"apiVersion":"v1","kind":"List","items":[]}`

	resources, err := ParseJSON([]byte(content), "empty-list.json")
	if err != nil {
		t.Fatalf("ParseJSON() error = %v", err)
	}

	if len(resources) != 0 {
		t.Errorf("Expected 0 resources for empty List, got %d: %v", len(resources), resources)
	}
}

func TestParseFileYAML(t *testing.T) {
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

func TestParseFileYML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deploy.yml")
	content := `apiVersion: v1
kind: Service
metadata:
  name: my-svc`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	resources, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}

	if resources[0].Kind != "Service" {
		t.Errorf("Kind = %q, expected Service", resources[0].Kind)
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

	os.WriteFile(filepath.Join(dir, "deploy.yaml"), []byte(`apiVersion: v1
kind: Pod
metadata:
  name: pod1`), 0644)

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

func TestParseNotFound(t *testing.T) {
	confirmFunc := func(url string) bool { return true }
	_, err := Parse("/nonexistent/path", false, confirmFunc)
	if err == nil {
		t.Error("Expected error for nonexistent path")
	}
}
