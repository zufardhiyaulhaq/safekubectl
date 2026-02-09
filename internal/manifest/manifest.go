package manifest

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

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
		if err != nil {
			return nil, err
		}
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

	return resources, nil
}

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
