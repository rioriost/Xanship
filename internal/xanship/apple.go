package xanship

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

func appleVolumeCreateArgs(v VolumePlan) []string {
	args := []string{"volume", "create"}
	for _, kv := range sortedMap(v.Labels) {
		args = append(args, "--label", kv)
	}
	return append(args, v.TargetName)
}

func appleNetworkCreateArgs(n NetworkPlan) []string {
	args := []string{"network", "create"}
	if n.Internal {
		args = append(args, "--internal")
	}
	for _, subnet := range n.Subnets {
		if strings.Contains(subnet, ":") {
			args = append(args, "--subnet-v6", subnet)
		} else {
			args = append(args, "--subnet", subnet)
		}
	}
	for _, kv := range sortedMap(n.Labels) {
		args = append(args, "--label", kv)
	}
	return append(args, n.TargetName)
}

func appleContainerArgs(action string, c ContainerPlan, nameOverride string) []string {
	args := []string{action}
	if action == "run" {
		args = append(args, "-d")
	}
	name := c.TargetName
	if nameOverride != "" {
		name = nameOverride
	}
	args = append(args, "--name", name)
	for _, env := range sortedCopy(c.Env) {
		args = append(args, "--env", env)
	}
	for _, kv := range sortedMap(c.Labels) {
		args = append(args, "--label", kv)
	}
	if c.User != "" {
		args = append(args, "--user", c.User)
	}
	if c.WorkingDir != "" {
		args = append(args, "--workdir", c.WorkingDir)
	}
	if c.TTY {
		args = append(args, "--tty")
	}
	if c.OpenStdin {
		args = append(args, "--interactive")
	}
	if c.Init {
		args = append(args, "--init")
	}
	if c.ReadOnlyRoot {
		args = append(args, "--read-only")
	}
	if c.MemoryBytes > 0 {
		args = append(args, "--memory", formatBytes(c.MemoryBytes))
	}
	if c.NanoCPUs > 0 {
		args = append(args, "--cpus", formatCPUs(c.NanoCPUs))
	}
	if c.ShmSizeBytes > 0 {
		args = append(args, "--shm-size", formatBytes(c.ShmSizeBytes))
	}
	for _, cap := range sortedCopy(c.CapAdd) {
		args = append(args, "--cap-add", normalizeCapability(cap))
	}
	for _, cap := range sortedCopy(c.CapDrop) {
		args = append(args, "--cap-drop", normalizeCapability(cap))
	}
	for _, dns := range sortedCopy(c.DNS) {
		args = append(args, "--dns", dns)
	}
	for _, dnsSearch := range sortedCopy(c.DNSSearch) {
		args = append(args, "--dns-search", dnsSearch)
	}
	for _, p := range c.Ports {
		args = append(args, "--publish", publishSpec(p))
	}
	for _, m := range c.Mounts {
		switch m.Type {
		case "volume":
			source := m.TargetName
			if source == "" {
				source = m.Source
			}
			args = append(args, "--mount", mountSpec("volume", source, m.Target, m.ReadOnly))
		case "bind":
			args = append(args, "--mount", mountSpec("bind", m.Source, m.Target, m.ReadOnly))
		case "tmpfs":
			args = append(args, "--tmpfs", m.Target)
		}
	}
	for _, n := range c.Networks {
		args = append(args, "--network", n.TargetName)
	}
	if len(c.Entrypoint) > 0 {
		args = append(args, "--entrypoint", c.Entrypoint[0])
	}
	args = append(args, c.Image)
	if len(c.Entrypoint) > 1 {
		args = append(args, c.Entrypoint[1:]...)
	}
	args = append(args, c.Command...)
	return args
}

func normalizeCapability(cap string) string {
	if cap == "" || cap == "ALL" || strings.HasPrefix(cap, "CAP_") {
		return cap
	}
	return "CAP_" + cap
}

func publishSpec(p PortPlan) string {
	host := p.HostPort
	if p.HostIP != "" && p.HostIP != "0.0.0.0" && p.HostIP != "::" {
		host = p.HostIP + ":" + host
	}
	proto := p.Protocol
	if proto == "" {
		proto = "tcp"
	}
	return fmt.Sprintf("%s:%s/%s", host, p.ContainerPort, proto)
}

func mountSpec(kind, source, target string, ro bool) string {
	parts := []string{"type=" + kind, "source=" + source, "target=" + target}
	if ro {
		parts = append(parts, "readonly")
	}
	return strings.Join(parts, ",")
}

func formatBytes(n int64) string {
	const gib = 1024 * 1024 * 1024
	const mib = 1024 * 1024
	if n%gib == 0 {
		return strconv.FormatInt(n/gib, 10) + "G"
	}
	if n%mib == 0 {
		return strconv.FormatInt(n/mib, 10) + "M"
	}
	return strconv.FormatInt(n, 10)
}

func formatCPUs(nano int64) string {
	v := float64(nano) / 1_000_000_000
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func sortedMap(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k, v := range m {
		out = append(out, k+"="+v)
	}
	sort.Strings(out)
	return out
}

func commandLine(name string, args []string) string {
	quoted := make([]string, 0, len(args)+1)
	quoted = append(quoted, shellQuote(name))
	for _, a := range args {
		quoted = append(quoted, shellQuote(a))
	}
	return strings.Join(quoted, " ")
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if strings.IndexFunc(s, func(r rune) bool {
		return !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || strings.ContainsRune("@%_+=:,./-", r))
	}) == -1 {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
