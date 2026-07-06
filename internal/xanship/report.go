package xanship

import (
	"fmt"
	"io"
)

func WriteMarkdownReport(w io.Writer, plan *Plan, report CompatibilityReport) {
	fmt.Fprintln(w, "# Xanship migration report")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- Containers: %d\n", len(plan.Containers))
	fmt.Fprintf(w, "- Volumes: %d\n", len(plan.Volumes))
	fmt.Fprintf(w, "- Networks: %d\n", len(plan.Networks))
	fmt.Fprintf(w, "- Images: %d\n", len(plan.Images))
	fmt.Fprintln(w)
	fmt.Fprintln(w, "## Containers")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| Source | Target | Image |")
	fmt.Fprintln(w, "| --- | --- | --- |")
	for _, c := range plan.Containers {
		fmt.Fprintf(w, "| `%s` | `%s` | `%s` |\n", c.SourceName, c.TargetName, c.Image)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "## Compatibility issues")
	fmt.Fprintln(w)
	if len(report.Issues) == 0 {
		fmt.Fprintln(w, "No issues found.")
		return
	}
	fmt.Fprintln(w, "| Severity | Component | Code | Message |")
	fmt.Fprintln(w, "| --- | --- | --- | --- |")
	for _, issue := range report.Issues {
		fmt.Fprintf(w, "| %s | `%s` | `%s` | %s |\n", issue.Severity, issue.Component, issue.Code, issue.Message)
	}
}
