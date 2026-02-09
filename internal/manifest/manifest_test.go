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
