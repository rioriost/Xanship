package xanship

import (
	"strings"
	"testing"
)

func TestAppleContainerArgs(t *testing.T) {
	c := ContainerPlan{
		TargetName:  "xanship-web",
		Image:       "nginx:latest",
		Env:         []string{"B=2", "A=1"},
		WorkingDir:  "/srv",
		Init:        true,
		MemoryBytes: 512 * 1024 * 1024,
		NanoCPUs:    1_500_000_000,
		Mounts: []MountPlan{
			{Type: "volume", Source: "data", TargetName: "xanship-data", Target: "/data", ReadOnly: true},
			{Type: "bind", Source: "/tmp/site", Target: "/usr/share/nginx/html"},
		},
		Ports:    []PortPlan{{HostPort: "8080", ContainerPort: "80", Protocol: "tcp"}},
		Networks: []NetworkAttachment{{TargetName: "xanship-net"}},
		Command:  []string{"nginx", "-g", "daemon off;"},
	}
	args := appleContainerArgs("run", c, "")
	line := strings.Join(args, "\x00")
	for _, want := range []string{
		"run\x00-d",
		"--name\x00xanship-web",
		"--env\x00A=1\x00--env\x00B=2",
		"--memory\x00512M",
		"--cpus\x001.5",
		"--mount\x00type=volume,source=xanship-data,target=/data,readonly",
		"--publish\x008080:80/tcp",
		"--network\x00xanship-net",
		"nginx:latest\x00nginx\x00-g\x00daemon off;",
	} {
		if !strings.Contains(line, want) {
			t.Fatalf("args missing %q in %#v", want, args)
		}
	}
}

func TestSanitizeName(t *testing.T) {
	got := sanitizeName("xanship-/my project_web/1")
	if got != "xanship-my-project_web-1" {
		t.Fatalf("sanitizeName() = %q", got)
	}
}

func TestConvertPortsDeduplicatesDockerDualStackBindings(t *testing.T) {
	cp := &ContainerPlan{}
	ports := convertPorts(map[string][]dockerPortBinding{
		"80/tcp": {
			{HostIP: "0.0.0.0", HostPort: "18080"},
			{HostIP: "::", HostPort: "18080"},
		},
	}, nil, nil, cp)
	if len(ports) != 1 {
		t.Fatalf("got %d ports, want 1: %#v", len(ports), ports)
	}
	if ports[0].HostPort != "18080" || ports[0].ContainerPort != "80" || ports[0].Protocol != "tcp" {
		t.Fatalf("unexpected port: %#v", ports[0])
	}
}

func TestAppleCompatibleLabelsSkipsValuesWithEquals(t *testing.T) {
	var warnings []string
	labels := appleCompatibleLabels(map[string]string{
		"ok":  "https://example.test/path",
		"url": "https://example.test/search?q=value",
	}, &warnings)
	if _, ok := labels["url"]; ok {
		t.Fatalf("label with '=' was not skipped: %#v", labels)
	}
	if labels["ok"] == "" {
		t.Fatalf("compatible label missing: %#v", labels)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %#v, want one warning", warnings)
	}
}
