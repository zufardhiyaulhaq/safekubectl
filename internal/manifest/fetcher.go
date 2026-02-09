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
