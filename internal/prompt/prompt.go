package prompt

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/zufardhiyaulhaq/safekubectl/internal/checker"
)

const (
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorReset  = "\033[0m"
)

// DisplayWarning shows the danger warning to the user
func DisplayWarning(result *checker.CheckResult, args []string) {
	DisplayWarningTo(os.Stdout, result, args)
}

// DisplayWarningTo writes the warning to the specified writer
func DisplayWarningTo(w io.Writer, result *checker.CheckResult, args []string) {
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s%s  DANGEROUS OPERATION DETECTED%s\n", colorYellow, warningIcon(), colorReset)
	fmt.Fprintf(w, "├── Operation: %s%s%s\n", colorRed, result.Operation, colorReset)
	fmt.Fprintf(w, "├── Resource:  %s\n", result.Resource)
	// Don't show namespace for node-scoped operations (cordon, uncordon, drain, taint)
	if !result.IsNodeScoped {
		fmt.Fprintf(w, "├── Namespace: %s\n", result.Namespace)
	}
	fmt.Fprintf(w, "├── Cluster:   %s\n", result.Cluster)
	fmt.Fprintf(w, "└── Command:   kubectl %s\n", strings.Join(args, " "))
	fmt.Fprintln(w)
}

// AskConfirmation prompts user for confirmation and returns true if confirmed
func AskConfirmation() bool {
	return AskConfirmationFrom(os.Stdin, os.Stdout)
}

// AskConfirmationFrom prompts for confirmation using the specified reader and writer
func AskConfirmationFrom(r io.Reader, w io.Writer) bool {
	reader := bufio.NewReader(r)
	fmt.Fprint(w, "Proceed? [y/N]: ")

	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}

// DisplayAborted shows the operation was aborted
func DisplayAborted() {
	DisplayAbortedTo(os.Stdout)
}

// DisplayAbortedTo writes the aborted message to the specified writer
func DisplayAbortedTo(w io.Writer) {
	fmt.Fprintln(w, "Operation aborted.")
}

// DisplayProceeding shows the operation is proceeding (warn-only mode)
func DisplayProceeding() {
	DisplayProceedingTo(os.Stdout)
}

// DisplayProceedingTo writes the proceeding message to the specified writer
func DisplayProceedingTo(w io.Writer) {
	fmt.Fprintln(w, "Proceeding with operation...")
	fmt.Fprintln(w)
}

// warningIcon returns the warning emoji/icon
func warningIcon() string {
	return "\u26A0\uFE0F "
}

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
