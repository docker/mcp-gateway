#!/usr/bin/env python3
"""
Docker MCP Complete Catalog Fetcher and Categorizer
Fetches the complete list of MCP servers from Docker's registry and organizes them into intuitive categories
"""

import os
import json
import yaml
import subprocess
from typing import Dict, List, Any, Optional
from dataclasses import dataclass, field
from enum import Enum

class MCPCategory(Enum):
    """Categories for organizing MCP servers"""
    CLOUD_INFRASTRUCTURE = "Cloud & Infrastructure"
    DATABASES = "Databases & Data Storage"
    MONITORING_OBSERVABILITY = "Monitoring & Observability"
    WEB_AUTOMATION = "Web Automation & Scraping"
    COMMUNICATION = "Communication & Collaboration"
    PAYMENT_FINANCE = "Payment & Finance"
    AI_ML = "AI & Machine Learning"
    SEARCH_DISCOVERY = "Search & Discovery"
    PRODUCTIVITY = "Productivity Tools"
    DEVELOPMENT = "Development Tools"
    SECURITY = "Security & Authentication"
    MEDIA = "Media & Content"
    FILESYSTEM = "File System & Storage"
    UNKNOWN = "Uncategorized"


@dataclass
class MCPServer:
    """MCP Server metadata"""
    name: str
    image: str
    description: str
    category: MCPCategory
    version: str = "latest"
    env_vars: List[str] = field(default_factory=list)
    tools: List[str] = field(default_factory=list)
    tags: List[str] = field(default_factory=list)
    
    def to_yaml_dict(self) -> Dict[str, Any]:
        """Convert to YAML-compatible dictionary"""
        config = {
            "image": self.image,
            "description": self.description,
            "version": self.version,
            "category": self.category.value
        }
        
        if self.env_vars:
            config["env"] = {var: f"${{{var}}}" for var in self.env_vars}
        
        if self.tools:
            config["tools"] = self.tools
        
        if self.tags:
            config["tags"] = self.tags
            
        return config


class MCPCatalogFetcher:
    """Fetches and categorizes complete MCP catalog from Docker"""
    
    def __init__(self):
        """Initialize with comprehensive server definitions"""
        self.servers = self._define_known_servers()
    
    def _categorize_server(self, name: str, description: str) -> MCPCategory:
        """
        Automatically categorize server based on name and description
        
        Args:
            name: Server name
            description: Server description
            
        Returns:
            Appropriate MCPCategory
        """
        name_lower = name.lower()
        desc_lower = description.lower()
        
        # Cloud & Infrastructure
        if any(x in name_lower for x in ['aws', 'azure', 'gcp', 'kubernetes', 'k8s', 'docker']):
            return MCPCategory.CLOUD_INFRASTRUCTURE
        
        # Databases
        if any(x in name_lower for x in ['postgres', 'mysql', 'mongodb', 'neo4j', 'redis', 'elastic', 'database', 'sql']):
            return MCPCategory.DATABASES
        
        # Monitoring
        if any(x in name_lower for x in ['grafana', 'newrelic', 'datadog', 'prometheus', 'monitoring', 'observability']):
            return MCPCategory.MONITORING_OBSERVABILITY
        
        # Web Automation
        if any(x in name_lower for x in ['playwright', 'puppeteer', 'selenium', 'scraping', 'browser', 'apify']):
            return MCPCategory.WEB_AUTOMATION
        
        # Communication
        if any(x in name_lower for x in ['slack', 'discord', 'teams', 'email', 'gmail', 'outlook']):
            return MCPCategory.COMMUNICATION
        
        # Payment & Finance
        if any(x in name_lower for x in ['stripe', 'payment', 'paypal', 'billing', 'invoice']):
            return MCPCategory.PAYMENT_FINANCE
        
        # AI & ML
        if any(x in name_lower for x in ['openai', 'anthropic', 'huggingface', 'langchain', 'llm', 'ai', 'ml']):
            return MCPCategory.AI_ML
        
        # Search
        if any(x in name_lower for x in ['search', 'brave', 'google', 'bing']):
            return MCPCategory.SEARCH_DISCOVERY
        
        # Productivity
        if any(x in name_lower for x in ['notion', 'trello', 'jira', 'asana', 'calendar', 'todo']):
            return MCPCategory.PRODUCTIVITY
        
        # Development
        if any(x in name_lower for x in ['github', 'gitlab', 'git', 'code', 'ci', 'cd', 'jenkins']):
            return MCPCategory.DEVELOPMENT
        
        # Security
        if any(x in name_lower for x in ['auth', 'security', 'vault', 'secrets', 'oauth']):
            return MCPCategory.SECURITY
        
        # Media
        if any(x in name_lower for x in ['youtube', 'video', 'audio', 'image', 'media']):
            return MCPCategory.MEDIA
        
        # Filesystem
        if any(x in name_lower for x in ['file', 'drive', 'storage', 's3', 'blob']):
            return MCPCategory.FILESYSTEM
        
        return MCPCategory.UNKNOWN
    
    def _define_known_servers(self) -> List[MCPServer]:
        """
        Define comprehensive list of known MCP servers
        
        Returns:
            List of MCPServer objects
        """
        servers = [
            # Cloud & Infrastructure
            MCPServer("aws", "mcp/aws", "AWS cloud services integration", 
                     MCPCategory.CLOUD_INFRASTRUCTURE,
                     env_vars=["AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_REGION"],
                     tools=["list_s3_buckets", "describe_ec2_instances", "invoke_lambda"],
                     tags=["cloud", "aws", "infrastructure"]),
            
            MCPServer("kubernetes", "mcp/kubernetes", "Kubernetes cluster management",
                     MCPCategory.CLOUD_INFRASTRUCTURE,
                     env_vars=["KUBECONFIG"],
                     tools=["get_pods", "describe_deployment", "apply_manifest"],
                     tags=["cloud", "k8s", "orchestration"]),
            
            # Databases
            MCPServer("postgres", "mcp/postgres", "PostgreSQL database queries and management",
                     MCPCategory.DATABASES,
                     env_vars=["DATABASE_URL"],
                     tools=["execute_query", "list_tables", "describe_table"],
                     tags=["database", "sql", "postgres"]),
            
            MCPServer("neo4j", "mcp/neo4j", "Neo4j graph database queries",
                     MCPCategory.DATABASES,
                     env_vars=["NEO4J_URI", "NEO4J_USERNAME", "NEO4J_PASSWORD"],
                     tools=["execute_cypher", "get_schema"],
                     tags=["database", "graph", "neo4j"]),
            
            MCPServer("elastic", "mcp/elastic", "Elasticsearch search and analytics",
                     MCPCategory.DATABASES,
                     env_vars=["ELASTIC_URL", "ELASTIC_API_KEY"],
                     tools=["search", "index_document", "delete_document"],
                     tags=["database", "search", "elasticsearch"]),
            
            MCPServer("mongodb", "mcp/mongodb", "MongoDB document database operations",
                     MCPCategory.DATABASES,
                     env_vars=["MONGODB_URI"],
                     tools=["find_documents", "insert_document", "update_document"],
                     tags=["database", "nosql", "mongodb"]),
            
            # Monitoring & Observability
            MCPServer("newrelic", "mcp/newrelic", "New Relic observability and monitoring",
                     MCPCategory.MONITORING_OBSERVABILITY,
                     env_vars=["NEW_RELIC_API_KEY"],
                     tools=["query_metrics", "get_alerts", "list_applications"],
                     tags=["monitoring", "observability", "apm"]),
            
            MCPServer("grafana", "mcp/grafana", "Grafana dashboard and metrics visualization",
                     MCPCategory.MONITORING_OBSERVABILITY,
                     env_vars=["GRAFANA_URL", "GRAFANA_API_KEY"],
                     tools=["query_datasource", "get_dashboard", "create_annotation"],
                     tags=["monitoring", "visualization", "metrics"]),
            
            MCPServer("datadog", "mcp/datadog", "Datadog monitoring and analytics",
                     MCPCategory.MONITORING_OBSERVABILITY,
                     env_vars=["DATADOG_API_KEY", "DATADOG_APP_KEY"],
                     tools=["query_metrics", "create_event", "get_monitors"],
                     tags=["monitoring", "observability", "logs"]),
            
            # Web Automation
            MCPServer("playwright", "mcp/playwright", "Browser automation and web scraping",
                     MCPCategory.WEB_AUTOMATION,
                     tools=["navigate", "screenshot", "fill_form", "click_element"],
                     tags=["automation", "browser", "testing"]),
            
            MCPServer("apify", "mcp/apify", "Apify web scraping and automation platform",
                     MCPCategory.WEB_AUTOMATION,
                     env_vars=["APIFY_TOKEN"],
                     tools=["run_actor", "get_dataset", "scrape_url"],
                     tags=["scraping", "automation", "data-extraction"]),
            
            MCPServer("puppeteer", "mcp/puppeteer", "Puppeteer browser automation",
                     MCPCategory.WEB_AUTOMATION,
                     tools=["navigate", "screenshot", "pdf_generation", "scrape"],
                     tags=["automation", "browser", "headless"]),
            
            # Communication
            MCPServer("slack", "mcp/slack", "Slack workspace integration",
                     MCPCategory.COMMUNICATION,
                     env_vars=["SLACK_BOT_TOKEN"],
                     tools=["send_message", "list_channels", "create_channel"],
                     tags=["communication", "messaging", "collaboration"]),
            
            MCPServer("discord", "mcp/discord", "Discord server integration",
                     MCPCategory.COMMUNICATION,
                     env_vars=["DISCORD_TOKEN"],
                     tools=["send_message", "create_channel", "manage_roles"],
                     tags=["communication", "gaming", "community"]),
            
            MCPServer("gmail", "mcp/gmail", "Gmail email operations",
                     MCPCategory.COMMUNICATION,
                     env_vars=["GMAIL_CREDENTIALS"],
                     tools=["send_email", "read_emails", "search_emails"],
                     tags=["email", "communication", "google"]),
            
            # Payment & Finance
            MCPServer("stripe", "mcp/stripe", "Stripe payment processing",
                     MCPCategory.PAYMENT_FINANCE,
                     env_vars=["STRIPE_API_KEY"],
                     tools=["create_customer", "create_payment", "list_charges"],
                     tags=["payment", "finance", "billing"]),
            
            # Search & Discovery
            MCPServer("brave-search", "mcp/brave-search", "Brave Search web search",
                     MCPCategory.SEARCH_DISCOVERY,
                     env_vars=["BRAVE_API_KEY"],
                     tools=["web_search", "news_search", "image_search"],
                     tags=["search", "web", "discovery"]),
            
            MCPServer("google-search", "mcp/google-search", "Google Search integration",
                     MCPCategory.SEARCH_DISCOVERY,
                     env_vars=["GOOGLE_API_KEY", "GOOGLE_CSE_ID"],
                     tools=["web_search", "image_search"],
                     tags=["search", "google", "discovery"]),
            
            # Productivity
            MCPServer("notion", "mcp/notion", "Notion workspace pages and databases",
                     MCPCategory.PRODUCTIVITY,
                     env_vars=["NOTION_API_KEY"],
                     tools=["query_database", "create_page", "update_page"],
                     tags=["productivity", "notes", "knowledge"]),
            
            MCPServer("jira", "mcp/jira", "Jira project management",
                     MCPCategory.PRODUCTIVITY,
                     env_vars=["JIRA_URL", "JIRA_API_TOKEN"],
                     tools=["create_issue", "update_issue", "search_issues"],
                     tags=["productivity", "project-management", "agile"]),
            
            # Development
            MCPServer("github", "mcp/github", "GitHub repository management",
                     MCPCategory.DEVELOPMENT,
                     env_vars=["GITHUB_TOKEN"],
                     tools=["search_repositories", "create_issue", "list_pull_requests"],
                     tags=["development", "git", "collaboration"]),
            
            MCPServer("gitlab", "mcp/gitlab", "GitLab repository and CI/CD",
                     MCPCategory.DEVELOPMENT,
                     env_vars=["GITLAB_TOKEN"],
                     tools=["create_merge_request", "trigger_pipeline", "get_issues"],
                     tags=["development", "git", "cicd"]),
            
            # Media
            MCPServer("youtube-transcript", "mcp/youtube-transcript", "YouTube video transcript extraction",
                     MCPCategory.MEDIA,
                     tools=["get_transcript", "search_videos"],
                     tags=["media", "video", "youtube"]),
            
            # Filesystem
            MCPServer("filesystem", "mcp/filesystem", "Local filesystem operations",
                     MCPCategory.FILESYSTEM,
                     tools=["read_file", "write_file", "list_directory", "search_files"],
                     tags=["filesystem", "storage", "files"]),
            
            MCPServer("google-drive", "mcp/google-drive", "Google Drive file management",
                     MCPCategory.FILESYSTEM,
                     env_vars=["GOOGLE_DRIVE_CREDENTIALS"],
                     tools=["list_files", "upload_file", "download_file", "create_folder"],
                     tags=["storage", "cloud", "google"]),
            
            MCPServer("s3", "mcp/s3", "AWS S3 object storage",
                     MCPCategory.FILESYSTEM,
                     env_vars=["AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY"],
                     tools=["list_objects", "upload_object", "download_object"],
                     tags=["storage", "cloud", "aws"]),
        ]
        
        return servers
    
    def fetch_from_docker_hub(self) -> List[MCPServer]:
        """
        Fetch MCP servers from Docker Hub using docker CLI
        
        Returns:
            List of discovered MCP servers
        """
        try:
            # Try to search Docker Hub for mcp namespace
            result = subprocess.run(
                ["docker", "search", "mcp/", "--limit", "100"],
                capture_output=True,
                text=True
            )
            
            if result.returncode == 0:
                # Parse output and create server objects
                lines = result.stdout.strip().split('\n')[1:]  # Skip header
                discovered = []
                
                for line in lines:
                    parts = line.split()
                    if len(parts) >= 2:
                        name = parts[0].replace('mcp/', '')
                        description = ' '.join(parts[1:])
                        category = self._categorize_server(name, description)
                        
                        discovered.append(MCPServer(
                            name=name,
                            image=parts[0],
                            description=description,
                            category=category
                        ))
                
                return discovered
        except Exception as e:
            print(f"Could not fetch from Docker Hub: {e}")
        
        return []
    
    def generate_categorized_catalog(self, 
                                    include_discovered: bool = False) -> Dict[str, Any]:
        """
        Generate categorized catalog YAML structure
        
        Args:
            include_discovered: Include servers discovered from Docker Hub
            
        Returns:
            Categorized catalog dictionary
        """
        servers = self.servers.copy()
        
        if include_discovered:
            discovered = self.fetch_from_docker_hub()
            servers.extend(discovered)
        
        # Organize by category
        categorized = {}
        for category in MCPCategory:
            category_servers = [s for s in servers if s.category == category]
            if category_servers:
                categorized[category.value] = {
                    server.name: server.to_yaml_dict()
                    for server in category_servers
                }
        
        catalog = {
            "version": "1.0",
            "name": "docker-mcp-complete-catalog",
            "description": "Complete Docker MCP Catalog organized by category",
            "categories": categorized,
            "metadata": {
                "total_servers": len(servers),
                "total_categories": len([c for c in categorized if categorized[c]]),
                "generated_by": "Docker MCP Catalog Fetcher"
            }
        }
        
        return catalog
    
    def generate_flat_catalog(self, category_filter: MCPCategory = None) -> Dict[str, Any]:
        """
        Generate flat catalog (compatible with docker mcp gateway)
        
        Args:
            category_filter: Only include servers from this category
            
        Returns:
            Flat catalog dictionary
        """
        servers = self.servers
        
        if category_filter:
            servers = [s for s in servers if s.category == category_filter]
        
        catalog = {
            "version": "1.0",
            "name": "docker-mcp-catalog",
            "description": "Docker MCP Server Catalog",
            "servers": {
                server.name: server.to_yaml_dict()
                for server in servers
            }
        }
        
        return catalog
    
    def save_catalog(self, catalog: Dict[str, Any], filename: str):
        """
        Save catalog to YAML file
        
        Args:
            catalog: Catalog dictionary
            filename: Output filename
        """
        with open(filename, 'w') as f:
            yaml.dump(catalog, f, default_flow_style=False, sort_keys=False)
        print(f"✅ Catalog saved to {filename}")
    
    def list_by_category(self) -> Dict[str, List[str]]:
        """
        Get servers grouped by category
        
        Returns:
            Dictionary mapping category to server names
        """
        categorized = {}
        for category in MCPCategory:
            servers = [s.name for s in self.servers if s.category == category]
            if servers:
                categorized[category.value] = servers
        return categorized


def main():
    """Main function demonstrating catalog fetching and organization"""
    fetcher = MCPCatalogFetcher()
    
    print("=== Docker MCP Complete Catalog Fetcher ===\n")
    
    # Show servers by category
    print("Available Servers by Category:")
    print("=" * 70)
    
    by_category = fetcher.list_by_category()
    for category, servers in by_category.items():
        print(f"\n{category} ({len(servers)} servers):")
        for server in servers:
            print(f"  • {server}")
    
    # Generate categorized catalog
    print("\n\n=== Generating Categorized Catalog ===")
    categorized_catalog = fetcher.generate_categorized_catalog()
    fetcher.save_catalog(categorized_catalog, "docker-mcp-categorized.yaml")
    
    # Generate flat catalog (compatible with docker mcp)
    print("\n=== Generating Flat Catalog ===")
    flat_catalog = fetcher.generate_flat_catalog()
    fetcher.save_catalog(flat_catalog, "docker-mcp-flat.yaml")
    
    # Generate category-specific catalogs
    print("\n=== Generating Category-Specific Catalogs ===")
    for category in [MCPCategory.DATABASES, MCPCategory.WEB_AUTOMATION, 
                     MCPCategory.DEVELOPMENT]:
        catalog = fetcher.generate_flat_catalog(category_filter=category)
        filename = f"docker-mcp-{category.name.lower().replace('_', '-')}.yaml"
        fetcher.save_catalog(catalog, filename)
    
    # Try to discover servers from Docker Hub
    print("\n=== Attempting Docker Hub Discovery ===")
    discovered = fetcher.fetch_from_docker_hub()
    if discovered:
        print(f"Discovered {len(discovered)} additional servers from Docker Hub")
        full_catalog = fetcher.generate_categorized_catalog(include_discovered=True)
        fetcher.save_catalog(full_catalog, "docker-mcp-complete-with-discovered.yaml")
    
    print("\n\n✅ Catalog generation complete!")
    print("\nUsage:")
    print("  # Import catalog")
    print("  docker mcp catalog import ./docker-mcp-flat.yaml")
    print("\n  # Start gateway with specific category")
    print("  docker mcp gateway run --catalog docker-mcp-databases.yaml --enable-all-servers")
    print("\n  # Start gateway with all servers")
    print("  docker mcp gateway run --catalog docker-mcp-flat.yaml --enable-all-servers")


if __name__ == "__main__":
    main()