package version

import (
	"fmt"
	"os"
	"strings"
)

// ContainerMetrics holds cgroup-based container resource usage.
type ContainerMetrics struct {
	MemoryUsageMB float64 `json:"memory_usage_mb,omitempty"`
	MemoryLimitMB float64 `json:"memory_limit_mb,omitempty"`
	CPUUsageNs    int64   `json:"cpu_usage_ns,omitempty"`
}

// ReadContainerMetrics reads cgroup stats when running inside a container.
func ReadContainerMetrics() *ContainerMetrics {
	cm := &ContainerMetrics{}
	hasData := false

	if val := readCgroupInt("/sys/fs/cgroup/memory.current"); val > 0 {
		cm.MemoryUsageMB = float64(val) / 1024 / 1024
		hasData = true
	} else if val := readCgroupInt("/sys/fs/cgroup/memory/memory.usage_in_bytes"); val > 0 {
		cm.MemoryUsageMB = float64(val) / 1024 / 1024
		hasData = true
	}

	if val := readCgroupInt("/sys/fs/cgroup/memory.max"); val > 0 && val < 1<<62 {
		cm.MemoryLimitMB = float64(val) / 1024 / 1024
		hasData = true
	} else if val := readCgroupInt("/sys/fs/cgroup/memory/memory.limit_in_bytes"); val > 0 && val < 1<<62 {
		cm.MemoryLimitMB = float64(val) / 1024 / 1024
		hasData = true
	}

	if content, err := os.ReadFile("/sys/fs/cgroup/cpu.stat"); err == nil {
		for _, line := range strings.Split(string(content), "\n") {
			if strings.HasPrefix(line, "usage_usec ") {
				var usec int64
				fmt.Sscanf(line, "usage_usec %d", &usec)
				cm.CPUUsageNs = usec * 1000
				hasData = true
				break
			}
		}
	} else if val := readCgroupInt("/sys/fs/cgroup/cpuacct/cpuacct.usage"); val > 0 {
		cm.CPUUsageNs = val
		hasData = true
	}

	if !hasData {
		return nil
	}
	return cm
}

func readCgroupInt(path string) int64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	s := strings.TrimSpace(string(data))
	var val int64
	fmt.Sscanf(s, "%d", &val)
	return val
}
