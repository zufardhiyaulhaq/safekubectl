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
