package xanship

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func PrintPlanSummary(w io.Writer, plan *Plan) {
	fmt.Fprintf(w, "containers: %d\n", len(plan.Containers))
	fmt.Fprintf(w, "volumes: %d\n", len(plan.Volumes))
	fmt.Fprintf(w, "networks: %d\n", len(plan.Networks))
	fmt.Fprintf(w, "images: %d\n", len(plan.Images))
	fmt.Fprintf(w, "warnings: %d\n", len(plan.Warnings))
	for _, c := range plan.Containers {
		fmt.Fprintf(w, "- %s -> %s (%s)\n", c.SourceName, c.TargetName, c.Image)
	}
}

func ValidatePlan(plan *Plan) error {
	if plan.Version != planVersion {
		return fmt.Errorf("unsupported plan version %q", plan.Version)
	}
	if len(plan.Containers) == 0 {
		return fmt.Errorf("plan has no containers")
	}
	report := AnalyzePlan(plan, nil)
	if report.HasErrors() {
		data, _ := json.MarshalIndent(report, "", "  ")
		return fmt.Errorf("plan has compatibility errors: %s", data)
	}
	return nil
}

func SetPlanValue(plan *Plan, selector, field, value string) error {
	switch field {
	case "target-name":
		for i := range plan.Containers {
			if plan.Containers[i].SourceName == selector || plan.Containers[i].TargetName == selector {
				plan.Containers[i].TargetName = sanitizeName(value)
				return nil
			}
		}
		return fmt.Errorf("container %q not found", selector)
	case "image":
		for i := range plan.Containers {
			if plan.Containers[i].SourceName == selector || plan.Containers[i].TargetName == selector {
				if strings.TrimSpace(value) == "" {
					return fmt.Errorf("image cannot be empty")
				}
				plan.Containers[i].Image = value
				return nil
			}
		}
		return fmt.Errorf("container %q not found", selector)
	default:
		return fmt.Errorf("unsupported plan field %q", field)
	}
}
