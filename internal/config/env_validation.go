package config

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// RequiredEnvVars lists the environment variables that must be set for the gateway to operate
var RequiredEnvVars = []string{
	"MCP_GATEWAY_PORT",
	"MCP_GATEWAY_DOMAIN",
	"MCP_GATEWAY_API_KEY",
}

// containerIDPattern validates that a container ID only contains valid characters (hex digits)
// Container IDs are 64 character hex strings, but short form (12 chars) is also valid
var containerIDPattern = regexp.MustCompile(`^[a-f0-9]{12,64}$`)

// EnvValidationResult holds the result of environment validation
type EnvValidationResult struct {
	IsContainerized    bool
	ContainerID        string
	DockerAccessible   bool
	MissingEnvVars     []string
	PortMapped         bool
	StdinInteractive   bool
	ValidationErrors   []string
	ValidationWarnings []string
}

// IsValid returns true if all critical validations passed
func (r *EnvValidationResult) IsValid() bool {
	return len(r.ValidationErrors) == 0
}

// Error returns a combined error message for all validation errors
func (r *EnvValidationResult) Error() string {
	if r.IsValid() {
		return ""
	}
	return fmt.Sprintf("Environment validation failed:\n  - %s", strings.Join(r.ValidationErrors, "\n  - "))
}

// ValidateExecutionEnvironment performs comprehensive validation of the execution environment
// It checks Docker accessibility, required environment variables, and containerization status
func ValidateExecutionEnvironment() *EnvValidationResult {
	result := &EnvValidationResult{}

	// Check if running in a containerized environment
	result.IsContainerized, result.ContainerID = detectContainerized()

	// Check Docker daemon accessibility
	result.DockerAccessible = checkDockerAccessible()
	if !result.DockerAccessible {
		result.ValidationErrors = append(result.ValidationErrors,
			"Docker daemon is not accessible. Ensure the Docker socket is mounted or Docker is running.")
	}

	// Check required environment variables
	result.MissingEnvVars = checkRequiredEnvVars()
	if len(result.MissingEnvVars) > 0 {
		result.ValidationErrors = append(result.ValidationErrors,
			fmt.Sprintf("Required environment variables not set: %s", strings.Join(result.MissingEnvVars, ", ")))
	}

	return result
}

// ValidateContainerizedEnvironment performs additional validation for containerized mode
// This is called by run_containerized.sh through the binary or by the Go code directly
func ValidateContainerizedEnvironment(containerID string) *EnvValidationResult {
	result := ValidateExecutionEnvironment()
	result.IsContainerized = true
	result.ContainerID = containerID

	if containerID == "" {
		result.ValidationErrors = append(result.ValidationErrors,
			"Container ID could not be determined. Are you running in a Docker container?")
		return result
	}

	// Validate port mapping
	port := os.Getenv("MCP_GATEWAY_PORT")
	if port != "" {
		portMapped, err := checkPortMapping(containerID, port)
		if err != nil {
			result.ValidationWarnings = append(result.ValidationWarnings,
				fmt.Sprintf("Could not verify port mapping: %v", err))
		} else if !portMapped {
			result.ValidationErrors = append(result.ValidationErrors,
				fmt.Sprintf("MCP_GATEWAY_PORT (%s) is not mapped to a host port. Use: -p <host_port>:%s", port, port))
		}
		result.PortMapped = portMapped
	}

	// Check if stdin is interactive (requires -i flag)
	result.StdinInteractive = checkStdinInteractive(containerID)
	if !result.StdinInteractive {
		result.ValidationErrors = append(result.ValidationErrors,
			"Container was not started with -i flag. Stdin is required for configuration input.")
	}

	return result
}

// detectContainerized checks if we're running inside a Docker container
// It examines /proc/self/cgroup to detect container environment and extract container ID
func detectContainerized() (bool, string) {
	file, err := os.Open("/proc/self/cgroup")
	if err != nil {
		// If we can't read cgroup, we're likely not in a container
		return false, ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// Docker containers have docker paths in cgroup
		if strings.Contains(line, "docker") || strings.Contains(line, "containerd") {
			// Extract container ID from the path
			// Format is typically: 0::/docker/<container_id>
			parts := strings.Split(line, "/")
			for i, part := range parts {
				if (part == "docker" || part == "containerd") && i+1 < len(parts) {
					containerID := parts[i+1]
					// Container IDs are 64 hex characters (or 12 for short form)
					if len(containerID) >= 12 {
						return true, containerID
					}
				}
			}
			// Found docker/containerd reference but couldn't extract ID
			return true, ""
		}
	}

	// Also check for .dockerenv file
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true, ""
	}

	return false, ""
}

// checkDockerAccessible verifies that the Docker daemon is accessible
func checkDockerAccessible() bool {
	// First check if the Docker socket exists
	socketPath := os.Getenv("DOCKER_HOST")
	if socketPath == "" {
		socketPath = "/var/run/docker.sock"
	} else {
		// Parse unix:// prefix if present
		socketPath = strings.TrimPrefix(socketPath, "unix://")
	}

	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		return false
	}

	// Try to run docker info to verify connectivity
	cmd := exec.Command("docker", "info")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// checkRequiredEnvVars checks if all required environment variables are set
func checkRequiredEnvVars() []string {
	var missing []string
	for _, envVar := range RequiredEnvVars {
		if os.Getenv(envVar) == "" {
			missing = append(missing, envVar)
		}
	}
	return missing
}

// validateContainerID validates that the container ID is safe to use in commands
// Container IDs should only contain lowercase hex characters (a-f, 0-9)
func validateContainerID(containerID string) error {
	if containerID == "" {
		return fmt.Errorf("container ID is empty")
	}
	if !containerIDPattern.MatchString(containerID) {
		return fmt.Errorf("container ID contains invalid characters: must be 12-64 hex characters")
	}
	return nil
}

// checkPortMapping uses docker inspect to verify that the specified port is mapped
func checkPortMapping(containerID, port string) (bool, error) {
	if err := validateContainerID(containerID); err != nil {
		return false, err
	}

	// Use docker inspect to get port bindings
	cmd := exec.Command("docker", "inspect", "--format", "{{json .NetworkSettings.Ports}}", containerID)
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("docker inspect failed: %w", err)
	}

	// Parse the port from the output
	portKey := fmt.Sprintf("%s/tcp", port)
	outputStr := string(output)

	// Check if the port is in the output with a host binding
	// The format is like: {"8000/tcp":[{"HostIp":"0.0.0.0","HostPort":"8000"}]}
	return strings.Contains(outputStr, portKey) && strings.Contains(outputStr, "HostPort"), nil
}

// checkStdinInteractive uses docker inspect to verify the container was started with -i flag
func checkStdinInteractive(containerID string) bool {
	if err := validateContainerID(containerID); err != nil {
		return false
	}

	// Use docker inspect to check stdin_open
	cmd := exec.Command("docker", "inspect", "--format", "{{.Config.OpenStdin}}", containerID)
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	return strings.TrimSpace(string(output)) == "true"
}

// GetGatewayPortFromEnv returns the MCP_GATEWAY_PORT value, parsed as int
func GetGatewayPortFromEnv() (int, error) {
	portStr := os.Getenv("MCP_GATEWAY_PORT")
	if portStr == "" {
		return 0, fmt.Errorf("MCP_GATEWAY_PORT environment variable not set")
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, fmt.Errorf("invalid MCP_GATEWAY_PORT value: %s", portStr)
	}

	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("MCP_GATEWAY_PORT must be between 1 and 65535, got %d", port)
	}

	return port, nil
}

// GetGatewayDomainFromEnv returns the MCP_GATEWAY_DOMAIN value
func GetGatewayDomainFromEnv() string {
	return os.Getenv("MCP_GATEWAY_DOMAIN")
}

// GetGatewayAPIKeyFromEnv returns the MCP_GATEWAY_API_KEY value
func GetGatewayAPIKeyFromEnv() string {
	return os.Getenv("MCP_GATEWAY_API_KEY")
}
