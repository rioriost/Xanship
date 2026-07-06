package xanship

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestAnalyzePlanDetectsPortConflictsAndBindWarnings(t *testing.T) {
	plan := &Plan{Containers: []ContainerPlan{
		{TargetName: "web", Image: "nginx", Ports: []PortPlan{{HostPort: "8080", ContainerPort: "80"}}, Mounts: []MountPlan{{Type: "bind", Source: "/tmp/site", Target: "/site"}}},
		{TargetName: "api", Image: "httpd", Ports: []PortPlan{{HostPort: "8080", ContainerPort: "80"}}},
	}}
	report := AnalyzePlan(plan, map[string]string{"8080/tcp": "other"})
	if !report.HasErrors() {
		t.Fatalf("expected compatibility errors: %#v", report)
	}
	if !hasIssue(report, "duplicate-port") || !hasIssue(report, "occupied-port") || !hasIssue(report, "bind-mount") {
		t.Fatalf("missing expected issues: %#v", report.Issues)
	}
}

func TestBindPolicyCopyToVolume(t *testing.T) {
	dc := dockerContainer{
		ID:     "123456789012",
		Name:   "/web",
		Config: dockerContainerConfig{Image: "nginx"},
		Mounts: []dockerMount{{Type: "bind", Source: "/tmp/site", Destination: "/usr/share/nginx/html", RW: true}},
	}
	cp, err := convertContainer(dc, "x-", BindPolicyCopyToVolume, newNameAllocator(), map[string]string{}, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if got := cp.Mounts[0].Type; got != "bind-volume" {
		t.Fatalf("mount type = %q, want bind-volume", got)
	}
	if cp.Mounts[0].TargetName == "" {
		t.Fatalf("target volume name was not generated")
	}
	if !strings.Contains(sourceMountSpec(VolumePlan{SourceKind: "bind", SourcePath: "/tmp/site"}), "type=bind") {
		t.Fatalf("bind source mount spec not generated")
	}
}

func TestBindPolicyFail(t *testing.T) {
	dc := dockerContainer{
		ID:     "123456789012",
		Name:   "/web",
		Config: dockerContainerConfig{Image: "nginx"},
		Mounts: []dockerMount{{Type: "bind", Source: "/tmp/site", Destination: "/site", RW: true}},
	}
	if _, err := convertContainer(dc, "x-", BindPolicyFail, newNameAllocator(), map[string]string{}, map[string]string{}); err == nil {
		t.Fatalf("expected bind policy failure")
	}
}

func TestComposeOrderingAndDependsOnParsing(t *testing.T) {
	containers := []ContainerPlan{
		{TargetName: "web", Compose: &ComposeInfo{Service: "web", DependsOn: parseComposeDependsOn("db:service_started, cache:service_healthy")}},
		{TargetName: "cache", Compose: &ComposeInfo{Service: "cache"}},
		{TargetName: "db", Compose: &ComposeInfo{Service: "db"}},
	}
	ordered := orderComposeContainers(containers)
	if ordered[0].Compose.Service == "web" {
		t.Fatalf("dependent service was not moved after dependencies: %#v", ordered)
	}
	if got := strings.Join(containers[0].Compose.DependsOn, ","); got != "cache,db" {
		t.Fatalf("depends_on parse = %q", got)
	}
}

func TestPlanSummaryValidateAndSet(t *testing.T) {
	plan := samplePlan()
	var out bytes.Buffer
	PrintPlanSummary(&out, plan)
	if !strings.Contains(out.String(), "containers: 1") {
		t.Fatalf("summary missing container count: %s", out.String())
	}
	if err := ValidatePlan(plan); err != nil {
		t.Fatalf("ValidatePlan() error = %v", err)
	}
	if err := SetPlanValue(plan, "web", "target-name", "new web"); err != nil {
		t.Fatal(err)
	}
	if plan.Containers[0].TargetName != "new-web" {
		t.Fatalf("target name not sanitized/set: %#v", plan.Containers[0])
	}
}

func TestMarkdownReportIncludesIssues(t *testing.T) {
	plan := samplePlan()
	report := AnalyzePlan(plan, map[string]string{"8080/tcp": "other"})
	var out bytes.Buffer
	WriteMarkdownReport(&out, plan, report)
	if !strings.Contains(out.String(), "# Xanship migration report") || !strings.Contains(out.String(), "occupied-port") {
		t.Fatalf("unexpected report: %s", out.String())
	}
}

func TestCompareVolumeEntries(t *testing.T) {
	result := CompareVolumeEntries(
		[]VolumeEntry{{Path: "a", Size: 1}, {Path: "b", Size: 2}},
		[]VolumeEntry{{Path: "a", Size: 2}, {Path: "c", Size: 3}},
	)
	if result.OK() {
		t.Fatalf("expected differences")
	}
	if len(result.SourceOnly) != 1 || result.SourceOnly[0] != "b" {
		t.Fatalf("source-only mismatch: %#v", result)
	}
	if len(result.TargetOnly) != 1 || result.TargetOnly[0] != "c" {
		t.Fatalf("target-only mismatch: %#v", result)
	}
	if len(result.SizeMismatch) != 1 || !strings.Contains(result.SizeMismatch[0], "a") {
		t.Fatalf("size mismatch missing: %#v", result)
	}
}

func TestParseVolumeEntries(t *testing.T) {
	entries := parseVolumeEntries("12 ./a.txt\n34 ./dir/file with space.txt\nbad line\n")
	if len(entries) != 2 {
		t.Fatalf("entries = %#v", entries)
	}
	if entries[0].Path != "a.txt" || entries[0].Size != 12 {
		t.Fatalf("first entry = %#v", entries[0])
	}
	if entries[1].Path != "dir/file with space.txt" || entries[1].Size != 34 {
		t.Fatalf("second entry = %#v", entries[1])
	}
}

func TestDecideImageTransfer(t *testing.T) {
	for _, image := range []string{"nginx:alpine", "ghcr.io/example/private:latest", "localhost:5000/app:dev"} {
		if got := DecideImageTransfer(image); got.Mode != ImageTransferPull {
			t.Fatalf("DecideImageTransfer(%q) = %#v", image, got)
		}
	}
	if got := DecideImageTransfer(""); got.Mode != ImageTransferSaveLoad {
		t.Fatalf("empty image decision = %#v", got)
	}
}

func TestRunPreflightJSON(t *testing.T) {
	plan := samplePlan()
	var buf bytes.Buffer
	if err := writeJSON(&buf, AnalyzePlan(plan, nil)); err != nil {
		t.Fatal(err)
	}
	var decoded CompatibilityReport
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizePolicies(t *testing.T) {
	if _, err := normalizeBindPolicy("bad"); err == nil {
		t.Fatalf("expected bad bind policy error")
	}
	if got, err := normalizeExistingPolicy(ExistingPolicyReplace); err != nil || got != ExistingPolicyReplace {
		t.Fatalf("normalizeExistingPolicy = %q, %v", got, err)
	}
}

func TestCommandRunnerPrependsDockerArgs(t *testing.T) {
	runner := commandRunner{dockerArgs: []string{"--host", "ssh://ubuntu@example"}}
	args := runner.commandArgs("docker", "ps")
	if strings.Join(args, " ") != "--host ssh://ubuntu@example ps" {
		t.Fatalf("docker args = %#v", args)
	}
	if got := dockerHostFromArgs(args); got != "ssh://ubuntu@example" {
		t.Fatalf("dockerHostFromArgs = %q", got)
	}
}

func hasIssue(report CompatibilityReport, code string) bool {
	for _, issue := range report.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func samplePlan() *Plan {
	return &Plan{
		Version: planVersion,
		Containers: []ContainerPlan{{
			SourceName: "web",
			TargetName: "x-web",
			Image:      "nginx:alpine",
			Ports:      []PortPlan{{HostPort: "8080", ContainerPort: "80", Protocol: "tcp"}},
		}},
	}
}
