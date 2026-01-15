package integration

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSerenaContainerBasic tests the Serena containers using the MCP test client
func TestSerenaContainerBasic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Serena container test in short mode")
	}

	// Check if Docker is available
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not available, skipping container test")
	}

	// Test cases for different containers
	testCases := []struct {
		name          string
		containerName string
		skipReason    string
	}{
		{
			name:          "Unified Container",
			containerName: "aw-serena:local",
			skipReason:    "Unified container not built",
		},
		{
			name:          "Go Container",
			containerName: "serena-go:local",
			skipReason:    "Go container not built",
		},
		{
			name:          "Python Container",
			containerName: "serena-python:local",
			skipReason:    "Python container not built",
		},
		{
			name:          "Java Container",
			containerName: "serena-java:local",
			skipReason:    "Java container not built",
		},
		{
			name:          "TypeScript Container",
			containerName: "serena-typescript:local",
			skipReason:    "TypeScript container not built",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Check if container exists locally
			cmd := exec.Command("docker", "image", "inspect", tc.containerName)
			if err := cmd.Run(); err != nil {
				t.Skip(tc.skipReason)
			}

			// Create temporary workspace
			tmpDir, err := os.MkdirTemp("", "serena-test-*")
			require.NoError(t, err)
			defer os.RemoveAll(tmpDir)

			// Create sample files for language detection
			createSampleProject(t, tmpDir)

			// Use the Python MCP client to test the container
			success := testSerenaContainer(t, tc.containerName, tmpDir)
			assert.True(t, success, "Serena container test should pass")
		})
	}
}

// TestSerenaWithMCPGateway tests Serena containers through the MCP Gateway
func TestSerenaWithMCPGateway(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Serena gateway integration test in short mode")
	}

	// Check if Docker is available
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not available, skipping test")
	}

	// Check if aw-serena container exists
	cmd := exec.Command("docker", "image", "inspect", "aw-serena:local")
	if err := cmd.Run(); err != nil {
		t.Skip("aw-serena:local container not built")
	}

	// Find the binary
	binaryPath := findBinary(t)
	t.Logf("Using binary: %s", binaryPath)

	// Create temporary workspace
	tmpDir, err := os.MkdirTemp("", "serena-gateway-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create sample project
	createSampleProject(t, tmpDir)

	// Create gateway config with Serena
	configContent := fmt.Sprintf(`{
  "mcpServers": {
    "serena": {
      "type": "stdio",
      "container": "aw-serena:local",
      "mounts": [
        "%s:/workspace:rw"
      ],
      "env": {
        "SERENA_PROJECT": "/workspace",
        "SERENA_CONTEXT": "codex"
      }
    }
  }
}`, tmpDir)

	configFile := filepath.Join(tmpDir, "config.json")
	err = os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	// Start the gateway with Serena
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	port := "13099"
	gatewayCmd := exec.CommandContext(ctx, binaryPath,
		"--config", configFile,
		"--listen", "127.0.0.1:"+port,
		"--routed",
	)

	var stdout, stderr bytes.Buffer
	gatewayCmd.Stdout = &stdout
	gatewayCmd.Stderr = &stderr

	err = gatewayCmd.Start()
	require.NoError(t, err, "Failed to start gateway")

	// Ensure cleanup
	defer func() {
		if gatewayCmd.Process != nil {
			gatewayCmd.Process.Kill()
		}
	}()

	// Wait for gateway to start
	time.Sleep(3 * time.Second)

	// Check if gateway is running
	if gatewayCmd.ProcessState != nil && gatewayCmd.ProcessState.Exited() {
		t.Logf("Gateway stdout: %s", stdout.String())
		t.Logf("Gateway stderr: %s", stderr.String())
		t.Fatal("Gateway exited prematurely")
	}

	t.Log("✓ Gateway started with Serena container")

	// TODO: Add more specific tests for MCP protocol interaction
	// For now, we just verify the gateway can start with Serena

	// Cleanup
	cancel()
	gatewayCmd.Wait()

	t.Log("✓ Gateway with Serena test completed")
}

// testSerenaContainer tests a Serena container using the Python MCP client
func testSerenaContainer(t *testing.T, containerImage, workspacePath string) bool {
	t.Helper()

	// Find the Python test client script
	clientScript := filepath.Join("test", "mcp-client", "mcp_client.py")
	if _, err := os.Stat(clientScript); err != nil {
		t.Logf("MCP client script not found: %s", clientScript)
		return false
	}

	// Run the test
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", clientScript, containerImage, workspacePath)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Log output for debugging
	t.Logf("Test output:\n%s", stderr.String())
	if stdout.Len() > 0 {
		t.Logf("Stdout:\n%s", stdout.String())
	}

	return err == nil
}

// createSampleProject creates sample files for language detection
func createSampleProject(t *testing.T, dir string) {
	t.Helper()

	// Create Go files
	goMod := filepath.Join(dir, "go.mod")
	err := os.WriteFile(goMod, []byte("module example.com/test\n\ngo 1.23\n"), 0644)
	require.NoError(t, err)

	mainGo := filepath.Join(dir, "main.go")
	err = os.WriteFile(mainGo, []byte("package main\n\nfunc main() {\n\tprintln(\"Hello\")\n}\n"), 0644)
	require.NoError(t, err)

	// Create Python files
	mainPy := filepath.Join(dir, "main.py")
	err = os.WriteFile(mainPy, []byte("print('Hello, World!')\n"), 0644)
	require.NoError(t, err)

	requirements := filepath.Join(dir, "requirements.txt")
	err = os.WriteFile(requirements, []byte("requests==2.31.0\n"), 0644)
	require.NoError(t, err)

	// Create TypeScript/Node files
	indexTs := filepath.Join(dir, "index.ts")
	err = os.WriteFile(indexTs, []byte("console.log('Hello, World!');\n"), 0644)
	require.NoError(t, err)

	packageJson := filepath.Join(dir, "package.json")
	err = os.WriteFile(packageJson, []byte(`{"name": "test", "version": "1.0.0"}`+"\n"), 0644)
	require.NoError(t, err)

	// Create Java files
	pomXml := filepath.Join(dir, "pom.xml")
	err = os.WriteFile(pomXml, []byte(`<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
    <modelVersion>4.0.0</modelVersion>
    <groupId>com.example</groupId>
    <artifactId>test</artifactId>
    <version>1.0.0</version>
</project>
`), 0644)
	require.NoError(t, err)

	mainJava := filepath.Join(dir, "Main.java")
	err = os.WriteFile(mainJava, []byte("public class Main { public static void main(String[] args) {} }\n"), 0644)
	require.NoError(t, err)
}
