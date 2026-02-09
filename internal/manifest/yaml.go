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
