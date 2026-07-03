package xanship

import "encoding/json"

type dockerContainer struct {
	ID              string                `json:"Id"`
	Name            string                `json:"Name"`
	Config          dockerContainerConfig `json:"Config"`
	HostConfig      dockerHostConfig      `json:"HostConfig"`
	NetworkSettings dockerNetworkSettings `json:"NetworkSettings"`
	Mounts          []dockerMount         `json:"Mounts"`
	State           dockerState           `json:"State"`
}

type dockerContainerConfig struct {
	Hostname     string            `json:"Hostname"`
	Image        string            `json:"Image"`
	Entrypoint   json.RawMessage   `json:"Entrypoint"`
	Cmd          []string          `json:"Cmd"`
	Env          []string          `json:"Env"`
	Labels       map[string]string `json:"Labels"`
	WorkingDir   string            `json:"WorkingDir"`
	User         string            `json:"User"`
	Tty          bool              `json:"Tty"`
	OpenStdin    bool              `json:"OpenStdin"`
	ExposedPorts map[string]any    `json:"ExposedPorts"`
	Healthcheck  any               `json:"Healthcheck"`
}

type dockerHostConfig struct {
	Binds          []string                       `json:"Binds"`
	PortBindings   map[string][]dockerPortBinding `json:"PortBindings"`
	RestartPolicy  dockerRestartPolicy            `json:"RestartPolicy"`
	Privileged     bool                           `json:"Privileged"`
	ReadonlyRootfs bool                           `json:"ReadonlyRootfs"`
	Init           *bool                          `json:"Init"`
	NetworkMode    string                         `json:"NetworkMode"`
	CapAdd         []string                       `json:"CapAdd"`
	CapDrop        []string                       `json:"CapDrop"`
	DNS            []string                       `json:"Dns"`
	DNSSearch      []string                       `json:"DnsSearch"`
	ExtraHosts     []string                       `json:"ExtraHosts"`
	Memory         int64                          `json:"Memory"`
	NanoCPUs       int64                          `json:"NanoCpus"`
	ShmSize        int64                          `json:"ShmSize"`
	LogConfig      map[string]any                 `json:"LogConfig"`
}

type dockerRestartPolicy struct {
	Name string `json:"Name"`
}

type dockerNetworkSettings struct {
	Ports    map[string][]dockerPortBinding           `json:"Ports"`
	Networks map[string]dockerNetworkEndpointSettings `json:"Networks"`
}

type dockerNetworkEndpointSettings struct {
	Aliases    []string `json:"Aliases"`
	IPAddress  string   `json:"IPAddress"`
	MacAddress string   `json:"MacAddress"`
}

type dockerPortBinding struct {
	HostIP   string `json:"HostIp"`
	HostPort string `json:"HostPort"`
}

type dockerMount struct {
	Type        string `json:"Type"`
	Name        string `json:"Name"`
	Source      string `json:"Source"`
	Destination string `json:"Destination"`
	Mode        string `json:"Mode"`
	RW          bool   `json:"RW"`
}

type dockerState struct {
	Running bool `json:"Running"`
}

type dockerVolume struct {
	Name    string            `json:"Name"`
	Driver  string            `json:"Driver"`
	Labels  map[string]string `json:"Labels"`
	Options map[string]string `json:"Options"`
}

type dockerNetwork struct {
	Name     string            `json:"Name"`
	Driver   string            `json:"Driver"`
	Internal bool              `json:"Internal"`
	Labels   map[string]string `json:"Labels"`
	IPAM     dockerIPAM        `json:"IPAM"`
}

type dockerIPAM struct {
	Config []dockerIPAMConfig `json:"Config"`
}

type dockerIPAMConfig struct {
	Subnet string `json:"Subnet"`
}
