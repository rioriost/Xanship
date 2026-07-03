package xanship

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

type AssessOptions struct {
	Containers     []string
	ComposeProject string
	All            bool
	TargetPrefix   string
}

func BuildPlan(ctx context.Context, runner commandRunner, opts AssessOptions) (*Plan, error) {
	refs, selection, err := selectContainerRefs(ctx, runner, opts)
	if err != nil {
		return nil, err
	}
	if len(refs) == 0 {
		return nil, fmt.Errorf("no Docker containers selected")
	}

	var dockerContainers []dockerContainer
	args := append([]string{"inspect"}, refs...)
	if err := runner.json(ctx, &dockerContainers, "docker", args...); err != nil {
		return nil, err
	}
	if len(dockerContainers) == 0 {
		return nil, fmt.Errorf("docker inspect returned no containers")
	}

	volumeNames, _ := listDockerObjects(ctx, runner, "volume")
	networkNames, _ := listDockerObjects(ctx, runner, "network")

	volumeSet := map[string]bool{}
	networkSet := map[string]bool{}
	imageSet := map[string]bool{}
	var warnings []string
	var containers []ContainerPlan

	nameAllocator := newNameAllocator()
	volumeTargets := map[string]string{}
	networkTargets := map[string]string{}

	sort.Slice(dockerContainers, func(i, j int) bool {
		return strings.TrimPrefix(dockerContainers[i].Name, "/") < strings.TrimPrefix(dockerContainers[j].Name, "/")
	})

	for _, dc := range dockerContainers {
		cp := convertContainer(dc, opts.TargetPrefix, nameAllocator, volumeTargets, networkTargets)
		imageSet[cp.Image] = true
		if !dc.State.Running {
			cp.Warnings = append(cp.Warnings, "source Docker container is not running")
		}
		if dc.HostConfig.RestartPolicy.Name != "" && dc.HostConfig.RestartPolicy.Name != "no" {
			cp.Warnings = append(cp.Warnings, "Docker restart policy is not migrated; configure lifecycle management separately")
		}
		if dc.HostConfig.Privileged {
			cp.Warnings = append(cp.Warnings, "privileged mode is not migrated")
		}
		if explicitDockerHostname(dc) {
			cp.Warnings = append(cp.Warnings, "custom Docker hostname is not migrated; Apple Container CLI does not expose hostname configuration")
		}
		if len(dc.HostConfig.ExtraHosts) > 0 {
			cp.Warnings = append(cp.Warnings, "extra_hosts entries are not migrated")
		}
		if dc.Config.Healthcheck != nil {
			cp.Warnings = append(cp.Warnings, "Docker healthcheck is not migrated")
		}
		if cp.Compose != nil {
			cp.Warnings = append(cp.Warnings, "Compose depends_on/start order is not available from docker inspect; containers are started in inspected name order")
		}
		for _, m := range cp.Mounts {
			if m.Type == "volume" && m.Source != "" {
				volumeSet[m.Source] = true
			}
		}
		for _, n := range cp.Networks {
			networkSet[n.SourceName] = true
		}
		containers = append(containers, cp)
		warnings = append(warnings, cp.Warnings...)
	}

	volumes, err := inspectVolumes(ctx, runner, volumeSet, volumeTargets)
	if err != nil {
		return nil, err
	}
	for i := range volumes {
		for _, c := range containers {
			for _, m := range c.Mounts {
				if m.Type == "volume" && m.Source == volumes[i].SourceName {
					volumes[i].MountedBy = append(volumes[i].MountedBy, c.TargetName)
				}
			}
		}
		sort.Strings(volumes[i].MountedBy)
	}

	networks, err := inspectNetworks(ctx, runner, networkSet, networkTargets)
	if err != nil {
		return nil, err
	}
	for _, n := range networks {
		if n.SourceName == "bridge" || n.SourceName == "host" || n.SourceName == "none" {
			continue
		}
	}

	images := make([]string, 0, len(imageSet))
	for image := range imageSet {
		images = append(images, image)
	}
	sort.Strings(images)
	sort.Strings(warnings)
	warnings = compactStrings(warnings)

	return &Plan{
		Version:   planVersion,
		CreatedAt: time.Now().UTC(),
		Source: SourceSummary{
			Selection:          selection,
			DockerContext:      dockerContext(ctx, runner),
			DockerVolumeCount:  len(volumeNames),
			DockerNetworkCount: len(networkNames),
			ContainerRefs:      refs,
		},
		Warnings:   warnings,
		Networks:   networks,
		Volumes:    volumes,
		Images:     images,
		Containers: containers,
	}, nil
}

func explicitDockerHostname(dc dockerContainer) bool {
	if dc.Config.Hostname == "" || len(dc.ID) < 12 {
		return false
	}
	return dc.Config.Hostname != dc.ID[:12]
}

func selectContainerRefs(ctx context.Context, runner commandRunner, opts AssessOptions) ([]string, string, error) {
	seen := map[string]bool{}
	var refs []string
	add := func(values ...string) {
		for _, v := range values {
			v = strings.TrimSpace(v)
			if v == "" || seen[v] {
				continue
			}
			seen[v] = true
			refs = append(refs, v)
		}
	}

	if len(opts.Containers) > 0 {
		add(opts.Containers...)
	}
	if opts.ComposeProject != "" {
		out, err := runner.output(ctx, "docker", "ps", "--filter", "label=com.docker.compose.project="+opts.ComposeProject, "--format", "{{.ID}}")
		if err != nil {
			return nil, "", err
		}
		add(strings.Fields(string(out))...)
	}
	if opts.All || (len(opts.Containers) == 0 && opts.ComposeProject == "") {
		out, err := runner.output(ctx, "docker", "ps", "--format", "{{.ID}}")
		if err != nil {
			return nil, "", err
		}
		add(strings.Fields(string(out))...)
	}
	sort.Strings(refs)
	switch {
	case opts.All || (len(opts.Containers) == 0 && opts.ComposeProject == ""):
		return refs, "all running Docker containers", nil
	case opts.ComposeProject != "" && len(opts.Containers) > 0:
		return refs, "explicit containers and Compose project " + opts.ComposeProject, nil
	case opts.ComposeProject != "":
		return refs, "Compose project " + opts.ComposeProject, nil
	default:
		return refs, "explicit containers", nil
	}
}

func listDockerObjects(ctx context.Context, runner commandRunner, kind string) ([]string, error) {
	out, err := runner.output(ctx, "docker", kind, "ls", "--format", "{{.Name}}")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, f := range strings.Fields(string(out)) {
		names = append(names, f)
	}
	sort.Strings(names)
	return names, nil
}

func dockerContext(ctx context.Context, runner commandRunner) string {
	out, err := runner.output(ctx, "docker", "context", "show")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func convertContainer(dc dockerContainer, prefix string, allocator *nameAllocator, volumeTargets, networkTargets map[string]string) ContainerPlan {
	sourceName := strings.TrimPrefix(dc.Name, "/")
	targetName := allocator.unique(sanitizeName(prefix + sourceName))
	cp := ContainerPlan{
		SourceID:     dc.ID,
		SourceName:   sourceName,
		TargetName:   targetName,
		Image:        dc.Config.Image,
		Entrypoint:   parseEntrypoint(dc.Config.Entrypoint),
		Command:      dc.Config.Cmd,
		Env:          append([]string(nil), dc.Config.Env...),
		WorkingDir:   dc.Config.WorkingDir,
		User:         dc.Config.User,
		Hostname:     dc.Config.Hostname,
		TTY:          dc.Config.Tty,
		OpenStdin:    dc.Config.OpenStdin,
		ReadOnlyRoot: dc.HostConfig.ReadonlyRootfs,
		MemoryBytes:  dc.HostConfig.Memory,
		NanoCPUs:     dc.HostConfig.NanoCPUs,
		ShmSizeBytes: dc.HostConfig.ShmSize,
		CapAdd:       sortedCopy(dc.HostConfig.CapAdd),
		CapDrop:      sortedCopy(dc.HostConfig.CapDrop),
		DNS:          sortedCopy(dc.HostConfig.DNS),
		DNSSearch:    sortedCopy(dc.HostConfig.DNSSearch),
	}
	cp.Labels = appleCompatibleLabels(dc.Config.Labels, &cp.Warnings)
	if dc.HostConfig.Init != nil {
		cp.Init = *dc.HostConfig.Init
	}
	if project := dc.Config.Labels["com.docker.compose.project"]; project != "" {
		cp.Compose = &ComposeInfo{
			Project:     project,
			Service:     dc.Config.Labels["com.docker.compose.service"],
			ContainerNo: dc.Config.Labels["com.docker.compose.container-number"],
		}
	}

	for _, m := range dc.Mounts {
		mp := MountPlan{Type: m.Type, Source: m.Name, Target: m.Destination, ReadOnly: !m.RW}
		switch m.Type {
		case "volume":
			mp.Source = m.Name
			if mp.Source != "" {
				if _, ok := volumeTargets[mp.Source]; !ok {
					volumeTargets[mp.Source] = sanitizeName(prefix + mp.Source)
				}
				mp.TargetName = volumeTargets[mp.Source]
			}
		case "bind":
			mp.Source = m.Source
		case "tmpfs":
			mp.Source = ""
		default:
			cp.Warnings = append(cp.Warnings, "mount type "+m.Type+" may not be supported by Apple Container")
		}
		cp.Mounts = append(cp.Mounts, mp)
	}
	sort.Slice(cp.Mounts, func(i, j int) bool {
		if cp.Mounts[i].Target == cp.Mounts[j].Target {
			return cp.Mounts[i].Source < cp.Mounts[j].Source
		}
		return cp.Mounts[i].Target < cp.Mounts[j].Target
	})

	cp.Ports = convertPorts(dc.NetworkSettings.Ports, dc.HostConfig.PortBindings, dc.Config.ExposedPorts, &cp)
	cp.Networks = convertNetworks(dc.NetworkSettings.Networks, prefix, networkTargets)
	return cp
}

func parseEntrypoint(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err == nil {
		return list
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil && single != "" {
		return []string{single}
	}
	return nil
}

func convertPorts(settings, host map[string][]dockerPortBinding, exposed map[string]any, cp *ContainerPlan) []PortPlan {
	source := settings
	if len(source) == 0 {
		source = host
	}
	var ports []PortPlan
	seen := map[string]bool{}
	for key, bindings := range source {
		containerPort, proto := splitPortProto(key)
		if len(bindings) == 0 {
			cp.Warnings = append(cp.Warnings, "exposed port "+key+" has no host publishing and is not migrated")
			continue
		}
		for _, b := range bindings {
			if b.HostPort == "" {
				cp.Warnings = append(cp.Warnings, "exposed port "+key+" has no host port and is not migrated")
				continue
			}
			dedupeKey := b.HostPort + "/" + containerPort + "/" + proto
			if seen[dedupeKey] {
				continue
			}
			seen[dedupeKey] = true
			ports = append(ports, PortPlan{
				HostIP: b.HostIP, HostPort: b.HostPort, ContainerPort: containerPort, Protocol: proto,
			})
		}
	}
	for key := range exposed {
		if _, ok := source[key]; !ok {
			cp.Warnings = append(cp.Warnings, "exposed port "+key+" has no host publishing and is not migrated")
		}
	}
	sort.Slice(ports, func(i, j int) bool {
		return ports[i].HostPort+ports[i].ContainerPort+ports[i].Protocol < ports[j].HostPort+ports[j].ContainerPort+ports[j].Protocol
	})
	return ports
}

func splitPortProto(key string) (string, string) {
	parts := strings.SplitN(key, "/", 2)
	if len(parts) == 1 {
		return parts[0], "tcp"
	}
	return parts[0], parts[1]
}

func convertNetworks(settings map[string]dockerNetworkEndpointSettings, prefix string, targets map[string]string) []NetworkAttachment {
	var out []NetworkAttachment
	for source, ep := range settings {
		if source == "bridge" || source == "host" || source == "none" {
			continue
		}
		if _, ok := targets[source]; !ok {
			targets[source] = sanitizeName(prefix + source)
		}
		out = append(out, NetworkAttachment{
			SourceName: source,
			TargetName: targets[source],
			Aliases:    compactStrings(ep.Aliases),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SourceName < out[j].SourceName })
	return out
}

func inspectVolumes(ctx context.Context, runner commandRunner, names map[string]bool, targets map[string]string) ([]VolumePlan, error) {
	if len(names) == 0 {
		return nil, nil
	}
	refs := sortedKeys(names)
	var dvs []dockerVolume
	args := append([]string{"volume", "inspect"}, refs...)
	if err := runner.json(ctx, &dvs, "docker", args...); err != nil {
		return nil, err
	}
	var volumes []VolumePlan
	for _, dv := range dvs {
		if dv.Driver != "" && dv.Driver != "local" {
			// Preserve a warning at plan level by encoding it as a label-free volume;
			// command generation still creates an Apple volume with the mapped name.
		}
		volumes = append(volumes, VolumePlan{
			SourceName: dv.Name,
			TargetName: targets[dv.Name],
			Labels:     copyStringMap(dv.Labels),
		})
	}
	sort.Slice(volumes, func(i, j int) bool { return volumes[i].SourceName < volumes[j].SourceName })
	return volumes, nil
}

func inspectNetworks(ctx context.Context, runner commandRunner, names map[string]bool, targets map[string]string) ([]NetworkPlan, error) {
	if len(names) == 0 {
		return nil, nil
	}
	refs := sortedKeys(names)
	var dns []dockerNetwork
	args := append([]string{"network", "inspect"}, refs...)
	if err := runner.json(ctx, &dns, "docker", args...); err != nil {
		return nil, err
	}
	var networks []NetworkPlan
	for _, dn := range dns {
		np := NetworkPlan{
			SourceName: dn.Name,
			TargetName: targets[dn.Name],
			Internal:   dn.Internal,
			Labels:     copyStringMap(dn.Labels),
		}
		for _, cfg := range dn.IPAM.Config {
			if cfg.Subnet != "" {
				np.Subnets = append(np.Subnets, cfg.Subnet)
			}
		}
		networks = append(networks, np)
	}
	sort.Slice(networks, func(i, j int) bool { return networks[i].SourceName < networks[j].SourceName })
	return networks, nil
}

func LoadPlan(path string) (*Plan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var plan Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, err
	}
	if plan.Version != planVersion {
		return nil, fmt.Errorf("unsupported plan version %q", plan.Version)
	}
	return &plan, nil
}

func SavePlan(path string, plan *Plan) error {
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

var invalidName = regexp.MustCompile(`[^A-Za-z0-9_.-]+`)
var repeatedHyphen = regexp.MustCompile(`-+`)
var appleLabelKey = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:/-]*$`)

func sanitizeName(name string) string {
	name = strings.TrimSpace(strings.TrimPrefix(name, "/"))
	name = invalidName.ReplaceAllString(name, "-")
	name = repeatedHyphen.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-_.")
	if name == "" {
		return "xanship"
	}
	return name
}

type nameAllocator struct {
	used map[string]int
}

func newNameAllocator() *nameAllocator {
	return &nameAllocator{used: map[string]int{}}
}

func (a *nameAllocator) unique(name string) string {
	if a.used[name] == 0 {
		a.used[name] = 1
		return name
	}
	a.used[name]++
	return fmt.Sprintf("%s-%d", name, a.used[name])
}

func copyStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func appleCompatibleLabels(in map[string]string, warnings *[]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		if !appleLabelKey.MatchString(k) || strings.ContainsAny(v, "=\n\r") {
			if warnings != nil {
				*warnings = append(*warnings, "Docker label "+k+" is not migrated because Apple Container label values cannot contain '=' or newlines")
			}
			continue
		}
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func sortedCopy(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func sortedKeys(in map[string]bool) []string {
	out := make([]string, 0, len(in))
	for k := range in {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func compactStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	sort.Strings(in)
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
