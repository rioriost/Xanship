package xanship

import (
	"fmt"
	"sort"
)

type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

type CompatibilityIssue struct {
	Severity  Severity `json:"severity"`
	Component string   `json:"component"`
	Code      string   `json:"code"`
	Message   string   `json:"message"`
}

type CompatibilityReport struct {
	Issues []CompatibilityIssue `json:"issues,omitempty"`
}

func (r CompatibilityReport) HasErrors() bool {
	for _, issue := range r.Issues {
		if issue.Severity == SeverityError {
			return true
		}
	}
	return false
}

func AnalyzePlan(plan *Plan, occupiedPorts map[string]string) CompatibilityReport {
	var issues []CompatibilityIssue
	seenPorts := map[string]string{}
	for _, warning := range plan.Warnings {
		issues = append(issues, issue(SeverityWarning, "plan", "plan-warning", warning))
	}
	for _, c := range plan.Containers {
		for _, warning := range c.Warnings {
			issues = append(issues, issue(SeverityWarning, c.TargetName, "container-warning", warning))
		}
		if c.Image == "" {
			issues = append(issues, issue(SeverityError, c.TargetName, "missing-image", "container image is empty"))
		}
		for _, m := range c.Mounts {
			if m.Type == "bind" {
				issues = append(issues, issue(SeverityWarning, c.TargetName, "bind-mount", "bind mount "+m.Source+" is host-specific; consider --bind-policy copy-to-volume"))
			}
		}
		for _, p := range c.Ports {
			key := p.HostPort + "/" + defaultProtocol(p.Protocol)
			if other := seenPorts[key]; other != "" {
				issues = append(issues, issue(SeverityError, c.TargetName, "duplicate-port", fmt.Sprintf("host port %s is also published by %s", key, other)))
			}
			if owner := occupiedPorts[key]; owner != "" {
				issues = append(issues, issue(SeverityError, c.TargetName, "occupied-port", fmt.Sprintf("host port %s is already used by %s", key, owner)))
			}
			seenPorts[key] = c.TargetName
		}
	}
	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].Severity == issues[j].Severity {
			return issues[i].Component+issues[i].Code < issues[j].Component+issues[j].Code
		}
		return severityRank(issues[i].Severity) > severityRank(issues[j].Severity)
	})
	return CompatibilityReport{Issues: issues}
}

func issue(severity Severity, component, code, message string) CompatibilityIssue {
	return CompatibilityIssue{Severity: severity, Component: component, Code: code, Message: message}
}

func severityRank(severity Severity) int {
	switch severity {
	case SeverityError:
		return 3
	case SeverityWarning:
		return 2
	default:
		return 1
	}
}

func defaultProtocol(proto string) string {
	if proto == "" {
		return "tcp"
	}
	return proto
}
