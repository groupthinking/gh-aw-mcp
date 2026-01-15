#!/usr/bin/env python3
"""
MCP Client Test Library

This module provides utilities for testing MCP servers via stdio transport.
"""

import json
import subprocess
import time
from typing import Any, Dict, List, Optional
import sys


class MCPClient:
    """Simple MCP client for testing stdio-based MCP servers."""
    
    def __init__(self, container_image: str, mount_path: str = None, env: Dict[str, str] = None):
        """
        Initialize MCP client with a container.
        
        Args:
            container_image: Docker container image for the MCP server
            mount_path: Optional path to mount into the container
            env: Optional environment variables
        """
        self.container_image = container_image
        self.mount_path = mount_path
        self.env = env or {}
        self.process = None
        self.request_id = 0
        
    def start(self):
        """Start the MCP server container."""
        cmd = ["docker", "run", "--rm", "-i"]
        
        # Add mounts
        if self.mount_path:
            cmd.extend(["-v", f"{self.mount_path}:/workspace:rw"])
            
        # Add environment variables
        for key, value in self.env.items():
            cmd.extend(["-e", f"{key}={value}"])
            
        # Add container image
        cmd.append(self.container_image)
        
        print(f"Starting MCP server: {' '.join(cmd)}", file=sys.stderr)
        
        self.process = subprocess.Popen(
            cmd,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            bufsize=1
        )
        
        # Give it a moment to start
        time.sleep(2)
        
    def stop(self):
        """Stop the MCP server container."""
        if self.process:
            self.process.terminate()
            try:
                self.process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                self.process.kill()
                self.process.wait()
                
    def send_request(self, method: str, params: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
        """
        Send a JSON-RPC request to the MCP server.
        
        Args:
            method: The JSON-RPC method name
            params: Optional parameters dictionary
            
        Returns:
            The JSON-RPC response
        """
        if not self.process:
            raise RuntimeError("MCP server not started")
            
        self.request_id += 1
        
        request = {
            "jsonrpc": "2.0",
            "id": self.request_id,
            "method": method
        }
        
        if params:
            request["params"] = params
            
        request_json = json.dumps(request) + "\n"
        print(f"Sending request: {request_json.strip()}", file=sys.stderr)
        
        self.process.stdin.write(request_json)
        self.process.stdin.flush()
        
        # Read response
        response_line = self.process.stdout.readline()
        if not response_line:
            stderr_output = self.process.stderr.read()
            raise RuntimeError(f"No response from server. stderr: {stderr_output}")
            
        print(f"Received response: {response_line.strip()}", file=sys.stderr)
        
        response = json.loads(response_line)
        return response
        
    def initialize(self, client_info: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
        """
        Send initialize request to MCP server.
        
        Args:
            client_info: Optional client information
            
        Returns:
            Server initialization response
        """
        params = {
            "protocolVersion": "2024-11-05",
            "capabilities": {},
            "clientInfo": client_info or {
                "name": "mcp-test-client",
                "version": "1.0.0"
            }
        }
        
        return self.send_request("initialize", params)
        
    def list_tools(self) -> List[Dict[str, Any]]:
        """
        List available tools from the MCP server.
        
        Returns:
            List of tool definitions
        """
        response = self.send_request("tools/list")
        return response.get("result", {}).get("tools", [])
        
    def call_tool(self, tool_name: str, arguments: Dict[str, Any]) -> Any:
        """
        Call a tool on the MCP server.
        
        Args:
            tool_name: Name of the tool to call
            arguments: Tool arguments
            
        Returns:
            Tool execution result
        """
        params = {
            "name": tool_name,
            "arguments": arguments
        }
        
        response = self.send_request("tools/call", params)
        return response.get("result")
        
    def __enter__(self):
        """Context manager entry."""
        self.start()
        return self
        
    def __exit__(self, exc_type, exc_val, exc_tb):
        """Context manager exit."""
        self.stop()


def test_mcp_server_basic(container_image: str, mount_path: str = None):
    """
    Basic test for MCP server connectivity and initialization.
    
    Args:
        container_image: Docker container image to test
        mount_path: Optional path to mount
        
    Returns:
        True if tests pass, False otherwise
    """
    print(f"\n=== Testing MCP Server: {container_image} ===\n")
    
    try:
        with MCPClient(container_image, mount_path) as client:
            # Test 1: Initialize
            print("Test 1: Initialize")
            init_response = client.initialize()
            
            if "result" not in init_response:
                print(f"❌ Initialize failed: {init_response}")
                return False
                
            print(f"✓ Initialize successful")
            print(f"  Server info: {init_response['result'].get('serverInfo', {})}")
            
            # Test 2: List tools
            print("\nTest 2: List tools")
            tools = client.list_tools()
            
            if not tools:
                print("⚠ No tools available (might be expected)")
            else:
                print(f"✓ Found {len(tools)} tools:")
                for tool in tools[:5]:  # Show first 5
                    print(f"  - {tool.get('name')}: {tool.get('description', 'No description')}")
                    
            print("\n✅ All tests passed!")
            return True
            
    except Exception as e:
        print(f"\n❌ Test failed with error: {e}")
        import traceback
        traceback.print_exc()
        return False


if __name__ == "__main__":
    import sys
    
    if len(sys.argv) < 2:
        print("Usage: python mcp_client.py <container_image> [mount_path]")
        sys.exit(1)
        
    container_image = sys.argv[1]
    mount_path = sys.argv[2] if len(sys.argv) > 2 else None
    
    success = test_mcp_server_basic(container_image, mount_path)
    sys.exit(0 if success else 1)
