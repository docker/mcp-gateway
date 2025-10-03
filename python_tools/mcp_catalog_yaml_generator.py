#!/usr/bin/env python3
"""
Docker MCP Catalog YAML Generator
Generates catalog YAML files with MCP servers from Docker MCP Toolkit
"""

import yaml
from typing import Dict, List, Any
from dataclasses import dataclass, asdict

@dataclass
class MCPServerConfig:
    """Configuration for an MCP server entry"""
    name: str
    image: str
    description: str
    version: str = "latest"
    env: Dict[str, str] = None
    tools: List[str] = None
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary format for YAML"""
        config = {
            "image": self.image,
            "description": self.description,
            "version": self.version
        }
        
        if self.env:
            config["env"] = self.env
        
        if self.tools:
            config["tools"] = self.tools
            
        return config


class MCPCatalogGenerator:
    """Generates MCP catalog YAML files"""
    
    def __init__(self):
        """Initialize with popular MCP servers from Docker Hub"""
        self.popular_servers = self._get_popular_servers()
    
    def _get_popular_servers(self) -> List[MCPServerConfig]:
        """
        Get list of popular MCP servers from Docker MCP Catalog
        
        Returns:
            List of MCPServerConfig objects
        """
        return [
            # Official Docker MCP Servers
            MCPServerConfig(
                name="github",
                image="mcp/github",
                description="GitHub integration for repository management, issues, PRs",
                version="latest",
                tools=["search_repositories", "create_issue", "list_pull_requests"]
            ),
            MCPServerConfig(
                name="stripe",
                image="mcp/stripe",
                description="Stripe payment processing and customer management",
                version="latest",
                env={"STRIPE_API_KEY": "sk_test_..."},
                tools=["create_customer", "create_payment", "list_charges"]
            ),
            MCPServerConfig(
                name="slack",
                image="mcp/slack",
                description="Slack workspace integration for messaging and channels",
                version="latest",
                env={"SLACK_BOT_TOKEN": "xoxb-..."},
                tools=["send_message", "list_channels", "create_channel"]
            ),
            MCPServerConfig(
                name="postgres",
                image="mcp/postgres",
                description="PostgreSQL database queries and management",
                version="latest",
                env={"DATABASE_URL": "postgresql://user:pass@host:5432/db"},
                tools=["execute_query", "list_tables", "describe_table"]
            ),
            MCPServerConfig(
                name="filesystem",
                image="mcp/filesystem",
                description="Local filesystem operations and file management",
                version="latest",
                tools=["read_file", "write_file", "list_directory", "search_files"]
            ),
            MCPServerConfig(
                name="brave-search",
                image="mcp/brave-search",
                description="Web search using Brave Search API",
                version="latest",
                env={"BRAVE_API_KEY": "BSA..."},
                tools=["web_search", "news_search", "image_search"]
            ),
            MCPServerConfig(
                name="youtube-transcript",
                image="mcp/youtube-transcript",
                description="YouTube video transcript extraction",
                version="latest",
                tools=["get_transcript", "search_videos"]
            ),
            MCPServerConfig(
                name="playwright",
                image="mcp/playwright",
                description="Browser automation and web scraping",
                version="latest",
                tools=["navigate", "screenshot", "fill_form", "click_element"]
            ),
            MCPServerConfig(
                name="neo4j",
                image="mcp/neo4j",
                description="Neo4j graph database queries",
                version="latest",
                env={"NEO4J_URI": "bolt://localhost:7687"},
                tools=["execute_cypher", "get_schema"]
            ),
            MCPServerConfig(
                name="elastic",
                image="mcp/elastic",
                description="Elasticsearch search and analytics",
                version="latest",
                env={"ELASTIC_URL": "http://localhost:9200"},
                tools=["search", "index_document", "delete_document"]
            ),
            MCPServerConfig(
                name="newrelic",
                image="mcp/newrelic",
                description="New Relic observability and monitoring",
                version="latest",
                env={"NEW_RELIC_API_KEY": "NRAK-..."},
                tools=["query_metrics", "get_alerts", "list_applications"]
            ),
            MCPServerConfig(
                name="grafana",
                image="mcp/grafana",
                description="Grafana dashboard and metrics visualization",
                version="latest",
                env={"GRAFANA_URL": "http://localhost:3000"},
                tools=["query_datasource", "get_dashboard", "create_annotation"]
            ),
            MCPServerConfig(
                name="notion",
                image="mcp/notion",
                description="Notion workspace pages and databases",
                version="latest",
                env={"NOTION_API_KEY": "secret_..."},
                tools=["query_database", "create_page", "update_page"]
            ),
            MCPServerConfig(
                name="aws",
                image="mcp/aws",
                description="AWS cloud services integration",
                version="latest",
                env={
                    "AWS_ACCESS_KEY_ID": "AKIA...",
                    "AWS_SECRET_ACCESS_KEY": "..."
                },
                tools=["list_s3_buckets", "describe_ec2_instances", "invoke_lambda"]
            ),
            MCPServerConfig(
                name="google-drive",
                image="mcp/google-drive",
                description="Google Drive file management",
                version="latest",
                tools=["list_files", "upload_file", "download_file", "create_folder"]
            ),
        ]
    
    def generate_catalog_yaml(self, 
                            server_names: List[str] = None,
                            include_all: bool = False) -> str:
        """
        Generate catalog YAML content
        
        Args:
            server_names: List of server names to include (None = all)
            include_all: If True, include all popular servers
            
        Returns:
            YAML string content
        """
        catalog = {
            "version": "1.0",
            "name": "custom-mcp-catalog",
            "description": "Custom MCP server catalog",
            "servers": {}
        }
        
        # Filter servers based on selection
        selected_servers = self.popular_servers
        if server_names and not include_all:
            selected_servers = [
                s for s in self.popular_servers 
                if s.name in server_names
            ]
        
        # Add servers to catalog
        for server in selected_servers:
            catalog["servers"][server.name] = server.to_dict()
        
        return yaml.dump(catalog, default_flow_style=False, sort_keys=False)
    
    def generate_with_existing_servers(self, 
                                      existing_servers: Dict[str, Any],
                                      mcp_toolkit_servers: List[str]) -> str:
        """
        Merge existing servers with MCP Toolkit servers
        
        Args:
            existing_servers: Dictionary of existing server configs
            mcp_toolkit_servers: List of MCP Toolkit server names to add
            
        Returns:
            YAML string content
        """
        catalog = {
            "version": "1.0",
            "name": "merged-mcp-catalog",
            "description": "Merged catalog with existing and MCP Toolkit servers",
            "servers": {}
        }
        
        # Add existing servers
        catalog["servers"].update(existing_servers)
        
        # Add selected MCP Toolkit servers
        for server in self.popular_servers:
            if server.name in mcp_toolkit_servers:
                catalog["servers"][server.name] = server.to_dict()
        
        return yaml.dump(catalog, default_flow_style=False, sort_keys=False)
    
    def list_available_servers(self) -> List[Dict[str, str]]:
        """
        List all available servers with descriptions
        
        Returns:
            List of dictionaries with server info
        """
        return [
            {
                "name": server.name,
                "image": server.image,
                "description": server.description
            }
            for server in self.popular_servers
        ]
    
    def save_catalog(self, content: str, filename: str = "docker-mcp.yaml"):
        """
        Save catalog YAML to file
        
        Args:
            content: YAML content string
            filename: Output filename
        """
        with open(filename, 'w') as f:
            f.write(content)
        print(f"Catalog saved to {filename}")


def main():
    """Main function demonstrating catalog generation"""
    generator = MCPCatalogGenerator()
    
    print("=== Docker MCP Catalog Generator ===\n")
    
    # List available servers
    print("Available MCP Toolkit Servers:")
    print("-" * 60)
    for server in generator.list_available_servers():
        print(f"{server['name']:20} - {server['description']}")
    print("\n")
    
    # Example 1: Generate catalog with all servers
    print("=== Example 1: All Servers ===")
    all_catalog = generator.generate_catalog_yaml(include_all=True)
    print(all_catalog)
    generator.save_catalog(all_catalog, "docker-mcp-all.yaml")
    
    # Example 2: Generate catalog with specific servers
    print("\n=== Example 2: Selected Servers ===")
    selected_servers = ["github", "stripe", "postgres", "playwright", "notion"]
    selected_catalog = generator.generate_catalog_yaml(server_names=selected_servers)
    print(selected_catalog)
    generator.save_catalog(selected_catalog, "docker-mcp-selected.yaml")
    
    # Example 3: Merge with existing servers
    print("\n=== Example 3: Merged with Existing Servers ===")
    existing = {
        "postgresql-mcp": {
            "image": "custom/postgresql-mcp",
            "description": "Custom PostgreSQL MCP server",
            "version": "1.0",
            "command": "node",
            "args": ["/users/bits/postgresql-mcp-server/build/index.js"]
        },
        "terminal-controller": {
            "image": "custom/terminal-controller",
            "description": "Terminal control MCP server",
            "version": "1.0",
            "command": "/Users/bits/terminal-controller-mcp/.venv/bin/python",
            "args": ["/Users/bits/terminal-controller-mcp/terminal_controller.py"]
        }
    }
    
    merged_catalog = generator.generate_with_existing_servers(
        existing,
        ["github", "youtube-transcript", "brave-search"]
    )
    print(merged_catalog)
    generator.save_catalog(merged_catalog, "docker-mcp-merged.yaml")
    
    print("\n✅ Catalog files generated successfully!")
    print("\nTo use these catalogs:")
    print("1. Import: docker mcp catalog import ./docker-mcp-merged.yaml")
    print("2. Start gateway: docker mcp gateway run --catalog docker-mcp-merged.yaml --enable-all-servers")


if __name__ == "__main__":
    main()