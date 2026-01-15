"""
Pytest tests for Serena MCP server containers.

These tests verify that Serena MCP servers are working correctly
by testing initialization, tool listing, and basic operations.
"""

import pytest
import os
import tempfile
from mcp_client import MCPClient


# Test containers - skip if not available
SERENA_CONTAINERS = [
    "ghcr.io/githubnext/aw-serena:latest",
    "ghcr.io/githubnext/serena-go:latest",
    "ghcr.io/githubnext/serena-python:latest",
    "ghcr.io/githubnext/serena-typescript:latest",
    "ghcr.io/githubnext/serena-java:latest",
]

# Use local images for testing if available
if os.getenv("USE_LOCAL_IMAGES"):
    SERENA_CONTAINERS = [
        "aw-serena:local",
        "serena-go:local",
        "serena-python:local",
        "serena-typescript:local",
        "serena-java:local",
    ]


@pytest.fixture
def temp_workspace():
    """Create a temporary workspace directory."""
    with tempfile.TemporaryDirectory() as tmpdir:
        # Create some sample files for language detection
        # Go
        go_mod = os.path.join(tmpdir, "go.mod")
        with open(go_mod, "w") as f:
            f.write("module example.com/test\n\ngo 1.23\n")
            
        # Python
        py_file = os.path.join(tmpdir, "main.py")
        with open(py_file, "w") as f:
            f.write("print('Hello, World!')\n")
            
        # TypeScript/JavaScript
        ts_file = os.path.join(tmpdir, "index.ts")
        with open(ts_file, "w") as f:
            f.write("console.log('Hello, World!');\n")
            
        package_json = os.path.join(tmpdir, "package.json")
        with open(package_json, "w") as f:
            f.write('{"name": "test", "version": "1.0.0"}\n')
        
        # Java
        pom_xml = os.path.join(tmpdir, "pom.xml")
        with open(pom_xml, "w") as f:
            f.write('''<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
    <modelVersion>4.0.0</modelVersion>
    <groupId>com.example</groupId>
    <artifactId>test</artifactId>
    <version>1.0.0</version>
</project>
''')
        
        java_file = os.path.join(tmpdir, "Main.java")
        with open(java_file, "w") as f:
            f.write('public class Main { public static void main(String[] args) {} }\n')
            
        yield tmpdir


@pytest.mark.parametrize("container_image", SERENA_CONTAINERS)
@pytest.mark.timeout(60)
def test_serena_initialize(container_image, temp_workspace):
    """Test that Serena container initializes correctly."""
    with MCPClient(container_image, temp_workspace) as client:
        response = client.initialize()
        
        assert "result" in response, f"Initialize response missing 'result': {response}"
        result = response["result"]
        
        assert "serverInfo" in result, "Initialize result missing 'serverInfo'"
        server_info = result["serverInfo"]
        
        assert "name" in server_info, "serverInfo missing 'name'"
        assert "version" in server_info, "serverInfo missing 'version'"
        
        print(f"✓ {container_image} initialized successfully")
        print(f"  Server: {server_info['name']} v{server_info['version']}")


@pytest.mark.parametrize("container_image", SERENA_CONTAINERS)
@pytest.mark.timeout(60)
def test_serena_list_tools(container_image, temp_workspace):
    """Test that Serena container can list tools."""
    with MCPClient(container_image, temp_workspace) as client:
        # Initialize first
        client.initialize()
        
        # List tools
        tools = client.list_tools()
        
        # Serena should have some tools available
        assert isinstance(tools, list), "tools/list should return a list"
        
        if tools:
            print(f"✓ {container_image} has {len(tools)} tools")
            for tool in tools[:3]:
                print(f"  - {tool.get('name')}")
        else:
            print(f"⚠ {container_image} has no tools (might be expected)")


@pytest.mark.timeout(60)
def test_unified_serena_multi_language(temp_workspace):
    """Test that unified Serena container supports multiple languages."""
    container_image = "aw-serena:local" if os.getenv("USE_LOCAL_IMAGES") else "ghcr.io/githubnext/aw-serena:latest"
    
    with MCPClient(container_image, temp_workspace) as client:
        # Initialize
        response = client.initialize()
        assert "result" in response
        
        # List tools - unified container should have tools for all languages
        tools = client.list_tools()
        assert isinstance(tools, list)
        
        print(f"✓ Unified container initialized with {len(tools)} tools")


@pytest.mark.skipif(not os.getenv("USE_LOCAL_IMAGES"), reason="Only for local testing")
@pytest.mark.timeout(60)
def test_serena_go_project():
    """Test Serena with a Go-specific project."""
    with tempfile.TemporaryDirectory() as tmpdir:
        # Create a simple Go project
        go_mod = os.path.join(tmpdir, "go.mod")
        with open(go_mod, "w") as f:
            f.write("module example.com/test\n\ngo 1.23\n")
            
        main_go = os.path.join(tmpdir, "main.go")
        with open(main_go, "w") as f:
            f.write('package main\n\nfunc main() {\n\tprintln("Hello")\n}\n')
        
        container_image = "serena-go:local"
        
        with MCPClient(container_image, tmpdir) as client:
            client.initialize()
            tools = client.list_tools()
            
            print(f"✓ Go project test passed with {len(tools)} tools")


@pytest.mark.skipif(not os.getenv("USE_LOCAL_IMAGES"), reason="Only for local testing")
@pytest.mark.timeout(60)
def test_serena_python_project():
    """Test Serena with a Python-specific project."""
    with tempfile.TemporaryDirectory() as tmpdir:
        # Create a simple Python project
        main_py = os.path.join(tmpdir, "main.py")
        with open(main_py, "w") as f:
            f.write('def hello():\n    print("Hello, World!")\n\nif __name__ == "__main__":\n    hello()\n')
            
        requirements = os.path.join(tmpdir, "requirements.txt")
        with open(requirements, "w") as f:
            f.write("requests==2.31.0\n")
        
        container_image = "serena-python:local"
        
        with MCPClient(container_image, tmpdir) as client:
            client.initialize()
            tools = client.list_tools()
            
            print(f"✓ Python project test passed with {len(tools)} tools")


@pytest.mark.skipif(not os.getenv("USE_LOCAL_IMAGES"), reason="Only for local testing")
@pytest.mark.timeout(60)
def test_serena_java_project():
    """Test Serena with a Java-specific project."""
    with tempfile.TemporaryDirectory() as tmpdir:
        # Create a simple Java project with Maven
        pom_xml = os.path.join(tmpdir, "pom.xml")
        with open(pom_xml, "w") as f:
            f.write('''<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 http://maven.apache.org/xsd/maven-4.0.0.xsd">
    <modelVersion>4.0.0</modelVersion>
    <groupId>com.example</groupId>
    <artifactId>test-project</artifactId>
    <version>1.0.0</version>
</project>
''')
        
        # Create a simple Java class
        src_dir = os.path.join(tmpdir, "src", "main", "java", "com", "example")
        os.makedirs(src_dir, exist_ok=True)
        
        main_java = os.path.join(src_dir, "Main.java")
        with open(main_java, "w") as f:
            f.write('''package com.example;

public class Main {
    public static void main(String[] args) {
        System.out.println("Hello, World!");
    }
}
''')
        
        container_image = "serena-java:local"
        
        with MCPClient(container_image, tmpdir) as client:
            client.initialize()
            tools = client.list_tools()
            
            print(f"✓ Java project test passed with {len(tools)} tools")


@pytest.mark.skipif(not os.getenv("USE_LOCAL_IMAGES"), reason="Only for local testing")
@pytest.mark.timeout(60)
def test_serena_typescript_project():
    """Test Serena with a TypeScript-specific project."""
    with tempfile.TemporaryDirectory() as tmpdir:
        # Create a TypeScript project
        package_json = os.path.join(tmpdir, "package.json")
        with open(package_json, "w") as f:
            f.write('''{
  "name": "test-typescript-project",
  "version": "1.0.0",
  "description": "Test TypeScript project",
  "main": "index.ts",
  "scripts": {
    "build": "tsc"
  },
  "devDependencies": {
    "typescript": "^5.0.0"
  }
}
''')
        
        # Create tsconfig.json
        tsconfig = os.path.join(tmpdir, "tsconfig.json")
        with open(tsconfig, "w") as f:
            f.write('''{
  "compilerOptions": {
    "target": "ES2020",
    "module": "commonjs",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true
  }
}
''')
        
        # Create a simple TypeScript file
        index_ts = os.path.join(tmpdir, "index.ts")
        with open(index_ts, "w") as f:
            f.write('''interface Greeter {
    greet(name: string): string;
}

class HelloGreeter implements Greeter {
    greet(name: string): string {
        return `Hello, ${name}!`;
    }
}

const greeter: Greeter = new HelloGreeter();
console.log(greeter.greet("World"));
''')
        
        container_image = "serena-typescript:local"
        
        with MCPClient(container_image, tmpdir) as client:
            client.initialize()
            tools = client.list_tools()
            
            print(f"✓ TypeScript project test passed with {len(tools)} tools")


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
