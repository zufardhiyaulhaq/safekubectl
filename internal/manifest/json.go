package manifest

import (
	"encoding/json"
	"fmt"
)

type kubeResourceJSON struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Items []kubeResourceJSON `json:"items,omitempty"`
}

func ParseJSON(content []byte, source string) ([]Resource, error) {
	var doc kubeResourceJSON
	if err := json.Unmarshal(content, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse JSON from %s: %w", source, err)
	}

	var resources []Resource

	// Handle List kind - return items, not the List itself
	if doc.Kind == "List" {
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
