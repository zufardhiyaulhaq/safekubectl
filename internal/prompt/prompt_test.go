package prompt

import (
	"bytes"
	"strings"
	"testing"

	"github.com/zufardhiyaulhaq/safekubectl/internal/checker"
	"github.com/zufardhiyaulhaq/safekubectl/internal/manifest"
)

func TestDisplayWarningTo(t *testing.T) {
	result := &checker.CheckResult{
		Operation: "delete",
		Resource:  "pod/nginx",
		Namespace: "production",
		Cluster:   "prod-cluster",
	}
	args := []string{"delete", "pod", "nginx", "-n", "production"}

	var buf bytes.Buffer
	DisplayWarningTo(&buf, result, args)
	output := buf.String()

	// Check that all expected elements are present
	expectedParts := []string{
		"DANGEROUS OPERATION DETECTED",
		"Operation:",
		"delete",
		"Resource:",
		"pod/nginx",
		"Namespace:",
		"production",
		"Cluster:",
		"prod-cluster",
		"Command:",
		"kubectl delete pod nginx -n production",
	}

	for _, part := range expectedParts {
		if !strings.Contains(output, part) {
			t.Errorf("expected output to contain %q, got:\n%s", part, output)
		}
	}

	// Check tree structure characters
	if !strings.Contains(output, "├──") {
		t.Error("expected output to contain tree structure '├──'")
	}
	if !strings.Contains(output, "└──") {
		t.Error("expected output to contain tree structure '└──'")
	}
}

func TestDisplayWarningToWithEmptyFields(t *testing.T) {
	result := &checker.CheckResult{
		Operation: "",
		Resource:  "",
		Namespace: "",
		Cluster:   "",
	}
	args := []string{}

	var buf bytes.Buffer
	DisplayWarningTo(&buf, result, args)
	output := buf.String()

	// Should still contain the header
	if !strings.Contains(output, "DANGEROUS OPERATION DETECTED") {
		t.Error("expected output to contain warning header")
	}
}

func TestAskConfirmationFrom(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"lowercase y", "y\n", true},
		{"uppercase Y", "Y\n", true},
		{"lowercase yes", "yes\n", true},
		{"uppercase YES", "YES\n", true},
		{"mixed case Yes", "Yes\n", true},
		{"n", "n\n", false},
		{"no", "no\n", false},
		{"empty", "\n", false},
		{"random text", "random\n", false},
		{"y with spaces", "  y  \n", true},
		{"yes with spaces", "  yes  \n", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := strings.NewReader(tt.input)
			var output bytes.Buffer

			result := AskConfirmationFrom(input, &output)
			if result != tt.expected {
				t.Errorf("AskConfirmationFrom(%q) = %v, expected %v", tt.input, result, tt.expected)
			}

			// Check that prompt was written
			if !strings.Contains(output.String(), "Proceed? [y/N]:") {
				t.Error("expected prompt to be written to output")
			}
		})
	}
}

func TestAskConfirmationFromReadError(t *testing.T) {
	// Empty reader will cause EOF error
	input := strings.NewReader("")
	var output bytes.Buffer

	result := AskConfirmationFrom(input, &output)
	if result != false {
		t.Error("expected false when read error occurs")
	}
}

func TestDisplayAbortedTo(t *testing.T) {
	var buf bytes.Buffer
	DisplayAbortedTo(&buf)
	output := buf.String()

	expected := "Operation aborted.\n"
	if output != expected {
		t.Errorf("DisplayAbortedTo() = %q, expected %q", output, expected)
	}
}

func TestDisplayProceedingTo(t *testing.T) {
	var buf bytes.Buffer
	DisplayProceedingTo(&buf)
	output := buf.String()

	if !strings.Contains(output, "Proceeding with operation...") {
		t.Errorf("expected output to contain 'Proceeding with operation...', got %q", output)
	}
}

func TestWarningIcon(t *testing.T) {
	icon := warningIcon()
	// Should contain the warning emoji character
	if !strings.Contains(icon, "\u26A0") {
		t.Errorf("expected warning icon to contain warning emoji, got %q", icon)
	}
}

func TestColorConstants(t *testing.T) {
	// Verify color constants are ANSI escape codes
	if !strings.HasPrefix(colorRed, "\033[") {
		t.Errorf("colorRed should be ANSI escape code, got %q", colorRed)
	}
	if !strings.HasPrefix(colorYellow, "\033[") {
		t.Errorf("colorYellow should be ANSI escape code, got %q", colorYellow)
	}
	if !strings.HasPrefix(colorReset, "\033[") {
		t.Errorf("colorReset should be ANSI escape code, got %q", colorReset)
	}
}

func TestDisplayWarningContainsColors(t *testing.T) {
	result := &checker.CheckResult{
		Operation: "delete",
		Resource:  "pod/nginx",
		Namespace: "production",
		Cluster:   "prod-cluster",
	}
	args := []string{"delete", "pod/nginx", "-n", "production"}

	var buf bytes.Buffer
	DisplayWarningTo(&buf, result, args)
	output := buf.String()

	// Check that colors are applied
	if !strings.Contains(output, colorYellow) {
		t.Error("expected output to contain yellow color code")
	}
	if !strings.Contains(output, colorRed) {
		t.Error("expected output to contain red color code")
	}
	if !strings.Contains(output, colorReset) {
		t.Error("expected output to contain color reset code")
	}
}

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
