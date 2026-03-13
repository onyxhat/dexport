package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	dnetwork "github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
)

// ComposeFile is the root of the docker-compose YAML document.
type ComposeFile struct {
	Services map[string]ServiceConfig `yaml:"services"`
	Networks map[string]NetworkConfig `yaml:"networks,omitempty"`
	Volumes  map[string]VolumeConfig  `yaml:"volumes,omitempty"`
}

// ServiceConfig mirrors the docker-compose service definition.
// Fields use omitempty so zero/false/nil/empty values are suppressed.
type ServiceConfig struct {
	Image           string                    `yaml:"image,omitempty"`
	ContainerName   string                    `yaml:"container_name,omitempty"`
	Hostname        string                    `yaml:"hostname,omitempty"`
	Domainname      string                    `yaml:"domainname,omitempty"`
	User            string                    `yaml:"user,omitempty"`
	WorkingDir      string                    `yaml:"working_dir,omitempty"`
	Entrypoint      []string                  `yaml:"entrypoint,omitempty"`
	Command         []string                  `yaml:"command,omitempty"`
	Environment     []string                  `yaml:"environment,omitempty"`
	Ports           []string                  `yaml:"ports,omitempty"`
	Volumes         []string                  `yaml:"volumes,omitempty"`
	Networks        map[string]ServiceNetwork `yaml:"networks,omitempty"`
	NetworkMode     string                    `yaml:"network_mode,omitempty"`
	Labels          map[string]string         `yaml:"labels,omitempty"`
	Restart         string                    `yaml:"restart,omitempty"`
	Privileged      bool                      `yaml:"privileged,omitempty"`
	ReadOnly        bool                      `yaml:"read_only,omitempty"`
	StdinOpen       bool                      `yaml:"stdin_open,omitempty"`
	Tty             bool                      `yaml:"tty,omitempty"`
	StopSignal      string                    `yaml:"stop_signal,omitempty"`
	StopGracePeriod string                    `yaml:"stop_grace_period,omitempty"`
	CapAdd          []string                  `yaml:"cap_add,omitempty"`
	CapDrop         []string                  `yaml:"cap_drop,omitempty"`
	SecurityOpt     []string                  `yaml:"security_opt,omitempty"`
	Devices         []string                  `yaml:"devices,omitempty"`
	DNS             []string                  `yaml:"dns,omitempty"`
	ExtraHosts      []string                  `yaml:"extra_hosts,omitempty"`
	Tmpfs           []string                  `yaml:"tmpfs,omitempty"`
	ShmSize         string                    `yaml:"shm_size,omitempty"`
	Sysctls         map[string]string         `yaml:"sysctls,omitempty"`
	GroupAdd        []string                  `yaml:"group_add,omitempty"`
	PidMode         string                    `yaml:"pid,omitempty"`
	IpcMode         string                    `yaml:"ipc,omitempty"`
	VolumesFrom     []string                  `yaml:"volumes_from,omitempty"`
	Logging         *LoggingConfig            `yaml:"logging,omitempty"`
	Healthcheck     *HealthcheckConfig        `yaml:"healthcheck,omitempty"`
	Ulimits         map[string]UlimitConfig   `yaml:"ulimits,omitempty"`
	MemLimit        string                    `yaml:"mem_limit,omitempty"`
	MemReservation  string                    `yaml:"mem_reservation,omitempty"`
	CPUs            string                    `yaml:"cpus,omitempty"`
	Init            *bool                     `yaml:"init,omitempty"`
}

// ServiceNetwork holds per-network configuration for a service.
type ServiceNetwork struct {
	Aliases []string `yaml:"aliases,omitempty"`
}

// NetworkConfig represents a top-level network definition.
type NetworkConfig struct {
	External bool   `yaml:"external,omitempty"`
	Driver   string `yaml:"driver,omitempty"`
	Name     string `yaml:"name,omitempty"`
}

// VolumeConfig represents a top-level named volume definition.
type VolumeConfig struct {
	External bool   `yaml:"external,omitempty"`
	Driver   string `yaml:"driver,omitempty"`
}

// LoggingConfig mirrors the compose logging block.
type LoggingConfig struct {
	Driver  string            `yaml:"driver,omitempty"`
	Options map[string]string `yaml:"options,omitempty"`
}

// HealthcheckConfig mirrors the compose healthcheck block.
type HealthcheckConfig struct {
	Test        []string `yaml:"test,omitempty"`
	Interval    string   `yaml:"interval,omitempty"`
	Timeout     string   `yaml:"timeout,omitempty"`
	Retries     int      `yaml:"retries,omitempty"`
	StartPeriod string   `yaml:"start_period,omitempty"`
	Disable     bool     `yaml:"disable,omitempty"`
}

// UlimitConfig represents a single ulimit with soft and hard values.
type UlimitConfig struct {
	Soft int64 `yaml:"soft"`
	Hard int64 `yaml:"hard"`
}

// convertToComposeFile converts a slice of inspected containers to a ComposeFile.
func convertToComposeFile(containers []container.InspectResponse) ComposeFile {
	topNetworks := make(map[string]NetworkConfig)
	topVolumes := make(map[string]VolumeConfig)

	cf := ComposeFile{
		Services: make(map[string]ServiceConfig),
	}

	for _, c := range containers {
		name := cleanName(c.Name)
		cf.Services[name] = containerToService(c, topNetworks, topVolumes)
	}

	if len(topNetworks) > 0 {
		cf.Networks = topNetworks
	}
	if len(topVolumes) > 0 {
		cf.Volumes = topVolumes
	}
	return cf
}

// containerToService converts a single InspectResponse to a ServiceConfig.
func containerToService(c container.InspectResponse, topNetworks map[string]NetworkConfig, topVolumes map[string]VolumeConfig) ServiceConfig {
	var svc ServiceConfig

	if c.Config != nil {
		cfg := c.Config
		svc.Image = cfg.Image
		svc.Hostname = cfg.Hostname
		svc.Domainname = cfg.Domainname
		svc.User = cfg.User
		svc.WorkingDir = cfg.WorkingDir
		svc.StopSignal = cfg.StopSignal
		svc.StdinOpen = cfg.OpenStdin
		svc.Tty = cfg.Tty
		svc.Environment = cfg.Env
		svc.Entrypoint = nilIfEmpty([]string(cfg.Entrypoint))
		svc.Command = nilIfEmpty([]string(cfg.Cmd))
		svc.Labels = filterLabels(cfg.Labels)
		svc.Healthcheck = mapHealthcheck(cfg.Healthcheck)
		if cfg.StopTimeout != nil && *cfg.StopTimeout > 0 {
			svc.StopGracePeriod = fmt.Sprintf("%ds", *cfg.StopTimeout)
		}
	}

	svc.ContainerName = cleanName(c.Name)

	if c.HostConfig != nil {
		hc := c.HostConfig
		svc.Restart = mapRestart(hc.RestartPolicy)
		svc.Privileged = hc.Privileged
		svc.ReadOnly = hc.ReadonlyRootfs
		svc.CapAdd = nilIfEmpty([]string(hc.CapAdd))
		svc.CapDrop = nilIfEmpty([]string(hc.CapDrop))
		svc.SecurityOpt = nilIfEmpty(hc.SecurityOpt)
		svc.DNS = nilIfEmpty(hc.DNS)
		svc.ExtraHosts = nilIfEmpty(hc.ExtraHosts)
		svc.GroupAdd = nilIfEmpty(hc.GroupAdd)
		svc.Sysctls = emptyMapToNil(hc.Sysctls)
		svc.Init = hc.Init
		svc.VolumesFrom = nilIfEmpty(hc.VolumesFrom)
		svc.Devices = mapDevices(hc.Devices)
		svc.Tmpfs = mapTmpfs(hc.Tmpfs)
		svc.Ulimits = mapUlimits(hc.Ulimits)
		svc.Logging = mapLogging(hc.LogConfig)
		svc.Ports = mapPorts(hc.PortBindings)

		if hc.ShmSize > 0 {
			svc.ShmSize = formatBytes(hc.ShmSize)
		}

		pidMode := string(hc.PidMode)
		if pidMode != "" {
			svc.PidMode = pidMode
		}

		ipcMode := string(hc.IpcMode)
		if ipcMode != "" && ipcMode != "private" {
			svc.IpcMode = ipcMode
		}

		svc.MemLimit, svc.MemReservation, svc.CPUs = mapResources(hc.Resources)

		// Network mode: emit for host/none/container:X; for bridge/custom use networks block
		nm := string(hc.NetworkMode)
		if nm == "host" || nm == "none" || strings.HasPrefix(nm, "container:") {
			svc.NetworkMode = nm
		}
	}

	svc.Volumes = mapVolumes(c.Mounts, topVolumes)

	if svc.NetworkMode == "" && c.NetworkSettings != nil {
		svc.Networks = mapNetworks(c.NetworkSettings.Networks, cleanName(c.Name), topNetworks)
	}

	return svc
}

// cleanName strips the leading "/" from a Docker container name.
func cleanName(name string) string {
	return strings.TrimPrefix(name, "/")
}

// nilIfEmpty returns nil when a string slice is empty, enabling omitempty.
func nilIfEmpty(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return s
}

// emptyMapToNil returns nil when a map is empty, enabling omitempty.
func emptyMapToNil(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	return m
}

// filterLabels removes internal Docker Compose labels that are not useful to
// re-declare (e.g. com.docker.compose.project, com.docker.compose.service).
func filterLabels(labels map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range labels {
		if strings.HasPrefix(k, "com.docker.compose.") {
			continue
		}
		result[k] = v
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// mapPorts converts a nat.PortMap to a sorted slice of compose port strings.
// Format: "[hostIP:]hostPort:containerPort[/proto]"
// The /tcp suffix and 0.0.0.0 bind address are omitted as they are defaults.
func mapPorts(bindings nat.PortMap) []string {
	if len(bindings) == 0 {
		return nil
	}

	var ports []string
	for port, hostBindings := range bindings {
		containerPort := port.Port()
		proto := port.Proto()

		for _, b := range hostBindings {
			var entry string
			if b.HostIP != "" && b.HostIP != "0.0.0.0" && b.HostIP != "::" {
				entry = fmt.Sprintf("%s:%s:%s", b.HostIP, b.HostPort, containerPort)
			} else {
				entry = fmt.Sprintf("%s:%s", b.HostPort, containerPort)
			}
			if proto != "tcp" {
				entry += "/" + proto
			}
			ports = append(ports, entry)
		}
	}

	sort.Strings(ports)
	return ports
}

// mapVolumes converts container mount points to compose volume strings.
// Named volumes are registered in topVolumes for the top-level volumes section.
func mapVolumes(mounts []container.MountPoint, topVolumes map[string]VolumeConfig) []string {
	if len(mounts) == 0 {
		return nil
	}

	var volumes []string
	for _, m := range mounts {
		switch m.Type {
		case mount.TypeBind:
			entry := m.Source + ":" + m.Destination
			if !m.RW {
				entry += ":ro"
			}
			volumes = append(volumes, entry)
		case mount.TypeVolume:
			if m.Name == "" {
				continue
			}
			entry := m.Name + ":" + m.Destination
			if !m.RW {
				entry += ":ro"
			}
			volumes = append(volumes, entry)
			topVolumes[m.Name] = VolumeConfig{}
		// TypeTmpfs is covered by HostConfig.Tmpfs
		}
	}
	return volumes
}

// builtinNetworks are Docker's built-in networks that don't need to be declared.
var builtinNetworks = map[string]bool{
	"bridge": true,
	"host":   true,
	"none":   true,
}

// mapNetworks converts endpoint settings to per-service networks and populates
// the top-level networks map with any non-built-in network names.
func mapNetworks(networks map[string]*dnetwork.EndpointSettings, containerName string, topNetworks map[string]NetworkConfig) map[string]ServiceNetwork {
	result := make(map[string]ServiceNetwork)
	for name, settings := range networks {
		if builtinNetworks[name] {
			continue
		}
		sn := ServiceNetwork{}
		if settings != nil {
			sn.Aliases = filterAliases(settings.Aliases, containerName)
		}
		result[name] = sn
		if _, exists := topNetworks[name]; !exists {
			topNetworks[name] = NetworkConfig{}
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// filterAliases removes Docker-generated aliases (container name and 12-char hex IDs).
func filterAliases(aliases []string, containerName string) []string {
	var result []string
	for _, a := range aliases {
		if a == containerName {
			continue
		}
		if len(a) == 12 && isHex(a) {
			continue
		}
		result = append(result, a)
	}
	return result
}

func isHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// mapRestart converts a RestartPolicy to a compose restart string.
func mapRestart(rp container.RestartPolicy) string {
	switch rp.Name {
	case container.RestartPolicyDisabled, "":
		return ""
	case container.RestartPolicyOnFailure:
		if rp.MaximumRetryCount > 0 {
			return fmt.Sprintf("on-failure:%d", rp.MaximumRetryCount)
		}
		return "on-failure"
	default:
		return string(rp.Name)
	}
}

// mapHealthcheck converts a container HealthConfig to a HealthcheckConfig.
func mapHealthcheck(hc *container.HealthConfig) *HealthcheckConfig {
	if hc == nil || len(hc.Test) == 0 {
		return nil
	}
	if len(hc.Test) == 1 && hc.Test[0] == "NONE" {
		return &HealthcheckConfig{Disable: true}
	}
	cfg := &HealthcheckConfig{
		Test:    hc.Test,
		Retries: hc.Retries,
	}
	if hc.Interval > 0 {
		cfg.Interval = hc.Interval.String()
	}
	if hc.Timeout > 0 {
		cfg.Timeout = hc.Timeout.String()
	}
	if hc.StartPeriod > 0 {
		cfg.StartPeriod = hc.StartPeriod.String()
	}
	return cfg
}

// mapLogging returns nil for the default json-file driver with no options,
// otherwise returns the logging configuration.
func mapLogging(lc container.LogConfig) *LoggingConfig {
	if (lc.Type == "" || lc.Type == "json-file") && len(lc.Config) == 0 {
		return nil
	}
	logging := &LoggingConfig{Driver: lc.Type}
	if len(lc.Config) > 0 {
		logging.Options = lc.Config
	}
	return logging
}

// mapResources extracts memory and CPU limits from the container resource config.
func mapResources(r container.Resources) (memLimit, memReservation, cpus string) {
	if r.Memory > 0 {
		memLimit = formatBytes(r.Memory)
	}
	if r.MemoryReservation > 0 {
		memReservation = formatBytes(r.MemoryReservation)
	}
	if r.NanoCPUs > 0 {
		cpus = strconv.FormatFloat(float64(r.NanoCPUs)/1e9, 'f', -1, 64)
	}
	return
}

// mapDevices converts DeviceMapping slice to compose device strings.
func mapDevices(devices []container.DeviceMapping) []string {
	if len(devices) == 0 {
		return nil
	}
	result := make([]string, len(devices))
	for i, d := range devices {
		entry := d.PathOnHost + ":" + d.PathInContainer
		// Omit cgroup permissions when they are the default "rwm"
		if d.CgroupPermissions != "" && d.CgroupPermissions != "rwm" {
			entry += ":" + d.CgroupPermissions
		}
		result[i] = entry
	}
	return result
}

// mapTmpfs converts the tmpfs map to a sorted slice of compose tmpfs strings.
func mapTmpfs(tmpfs map[string]string) []string {
	if len(tmpfs) == 0 {
		return nil
	}
	result := make([]string, 0, len(tmpfs))
	for path, opts := range tmpfs {
		if opts != "" {
			result = append(result, path+":"+opts)
		} else {
			result = append(result, path)
		}
	}
	sort.Strings(result)
	return result
}

// mapUlimits converts a slice of Ulimit to a map for the compose ulimits block.
func mapUlimits(ulimits []*container.Ulimit) map[string]UlimitConfig {
	if len(ulimits) == 0 {
		return nil
	}
	result := make(map[string]UlimitConfig, len(ulimits))
	for _, u := range ulimits {
		result[u.Name] = UlimitConfig{Soft: u.Soft, Hard: u.Hard}
	}
	return result
}

// formatBytes converts a byte count to a human-readable compose size string.
func formatBytes(n int64) string {
	const (
		_  = iota
		KB = 1 << (10 * iota)
		MB
		GB
	)
	switch {
	case n >= GB && n%GB == 0:
		return fmt.Sprintf("%dg", n/GB)
	case n >= MB && n%MB == 0:
		return fmt.Sprintf("%dm", n/MB)
	case n >= KB && n%KB == 0:
		return fmt.Sprintf("%dk", n/KB)
	default:
		return fmt.Sprintf("%d", n)
	}
}
