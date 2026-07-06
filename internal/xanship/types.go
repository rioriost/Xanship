package xanship

import "time"

const planVersion = "xanship/v1"

type Plan struct {
	Version    string          `json:"version"`
	CreatedAt  time.Time       `json:"created_at"`
	Source     SourceSummary   `json:"source"`
	Options    PlanOptions     `json:"options,omitempty"`
	Warnings   []string        `json:"warnings,omitempty"`
	Networks   []NetworkPlan   `json:"networks,omitempty"`
	Volumes    []VolumePlan    `json:"volumes,omitempty"`
	Images     []string        `json:"images,omitempty"`
	Containers []ContainerPlan `json:"containers"`
}

type PlanOptions struct {
	BindPolicy     string `json:"bind_policy,omitempty"`
	ExistingPolicy string `json:"existing_policy,omitempty"`
}

type SourceSummary struct {
	Selection          string   `json:"selection"`
	DockerContext      string   `json:"docker_context,omitempty"`
	DockerVolumeCount  int      `json:"docker_volume_count"`
	DockerNetworkCount int      `json:"docker_network_count"`
	ContainerRefs      []string `json:"container_refs"`
}

type NetworkPlan struct {
	SourceName string            `json:"source_name"`
	TargetName string            `json:"target_name"`
	Internal   bool              `json:"internal,omitempty"`
	Subnets    []string          `json:"subnets,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
}

type VolumePlan struct {
	SourceName string            `json:"source_name"`
	SourceKind string            `json:"source_kind,omitempty"`
	SourcePath string            `json:"source_path,omitempty"`
	TargetName string            `json:"target_name"`
	Labels     map[string]string `json:"labels,omitempty"`
	MountedBy  []string          `json:"mounted_by,omitempty"`
}

type ContainerPlan struct {
	SourceID     string              `json:"source_id"`
	SourceName   string              `json:"source_name"`
	TargetName   string              `json:"target_name"`
	Image        string              `json:"image"`
	Entrypoint   []string            `json:"entrypoint,omitempty"`
	Command      []string            `json:"command,omitempty"`
	Env          []string            `json:"env,omitempty"`
	Labels       map[string]string   `json:"labels,omitempty"`
	WorkingDir   string              `json:"working_dir,omitempty"`
	User         string              `json:"user,omitempty"`
	Hostname     string              `json:"hostname,omitempty"`
	TTY          bool                `json:"tty,omitempty"`
	OpenStdin    bool                `json:"open_stdin,omitempty"`
	Init         bool                `json:"init,omitempty"`
	ReadOnlyRoot bool                `json:"read_only_root,omitempty"`
	MemoryBytes  int64               `json:"memory_bytes,omitempty"`
	NanoCPUs     int64               `json:"nano_cpus,omitempty"`
	ShmSizeBytes int64               `json:"shm_size_bytes,omitempty"`
	CapAdd       []string            `json:"cap_add,omitempty"`
	CapDrop      []string            `json:"cap_drop,omitempty"`
	DNS          []string            `json:"dns,omitempty"`
	DNSSearch    []string            `json:"dns_search,omitempty"`
	Mounts       []MountPlan         `json:"mounts,omitempty"`
	Ports        []PortPlan          `json:"ports,omitempty"`
	Networks     []NetworkAttachment `json:"networks,omitempty"`
	Compose      *ComposeInfo        `json:"compose,omitempty"`
	Warnings     []string            `json:"warnings,omitempty"`
}

type ComposeInfo struct {
	Project     string   `json:"project,omitempty"`
	Service     string   `json:"service,omitempty"`
	ContainerNo string   `json:"container_no,omitempty"`
	DependsOn   []string `json:"depends_on,omitempty"`
}

type MountPlan struct {
	Type       string `json:"type"`
	Source     string `json:"source,omitempty"`
	Target     string `json:"target"`
	ReadOnly   bool   `json:"read_only,omitempty"`
	TargetName string `json:"target_name,omitempty"`
}

type PortPlan struct {
	HostIP        string `json:"host_ip,omitempty"`
	HostPort      string `json:"host_port"`
	ContainerPort string `json:"container_port"`
	Protocol      string `json:"protocol"`
}

type NetworkAttachment struct {
	SourceName string   `json:"source_name"`
	TargetName string   `json:"target_name"`
	Aliases    []string `json:"aliases,omitempty"`
}
