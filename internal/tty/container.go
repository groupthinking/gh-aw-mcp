package tty

import (
	"os"
	"strings"
	
	"github.com/githubnext/gh-aw-mcpg/internal/logger"
)

var logContainer = logger.New("tty:container")

// IsRunningInContainer detects if the current process is running inside a container
func IsRunningInContainer() bool {
	logContainer.Print("Detecting container environment")
	
	// Method 1: Check for /.dockerenv file (Docker-specific)
	if _, err := os.Stat("/.dockerenv"); err == nil {
		logContainer.Print("Container detected via /.dockerenv file")
		return true
	}

	// Method 2: Check /proc/1/cgroup for container indicators
	data, err := os.ReadFile("/proc/1/cgroup")
	if err == nil {
		content := string(data)
		if strings.Contains(content, "docker") ||
			strings.Contains(content, "containerd") ||
			strings.Contains(content, "kubepods") ||
			strings.Contains(content, "lxc") {
			logContainer.Print("Container detected via /proc/1/cgroup markers")
			return true
		}
	}

	// Method 3: Check environment variable (set by Dockerfile)
	if os.Getenv("RUNNING_IN_CONTAINER") == "true" {
		logContainer.Print("Container detected via RUNNING_IN_CONTAINER env var")
		return true
	}

	logContainer.Print("No container environment detected")
	return false
}
