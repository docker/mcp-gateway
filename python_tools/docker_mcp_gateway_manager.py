#!/usr/bin/env python3
"""
Docker MCP Gateway Manager - Streamlit GUI
Simple GUI to manage Docker MCP Gateway, select servers, and configure catalogs
"""

import streamlit as st
import subprocess
import yaml
import json
import os
import signal
from pathlib import Path
from typing import Dict, List, Any, Optional
from dataclasses import dataclass, asdict
from enum import Enum

# Set page config
st.set_page_config(
    page_title="Docker MCP Gateway Manager",
    page_icon="🐳",
    layout="wide"
)

class MCPCategory(Enum):
    """Categories for organizing MCP servers"""
    CLOUD_INFRASTRUCTURE = "☁️ Cloud & Infrastructure"
    DATABASES = "🗄️ Databases & Data Storage"
    MONITORING_OBSERVABILITY = "📊 Monitoring & Observability"
    WEB_AUTOMATION = "🌐 Web Automation & Scraping"
    COMMUNICATION = "💬 Communication & Collaboration"
    PAYMENT_FINANCE = "💳 Payment & Finance"
    AI_ML = "🤖 AI & Machine Learning"
    SEARCH_DISCOVERY = "🔍 Search & Discovery"
    PRODUCTIVITY = "📋 Productivity Tools"
    DEVELOPMENT = "👨‍💻 Development Tools"
    SECURITY = "🔒 Security & Authentication"
    MEDIA = "🎬 Media & Content"
    FILESYSTEM = "📁 File System & Storage"


@dataclass
class MCPServer:
    """MCP Server metadata"""
    name: str
    image: str
    description: str
    category: MCPCategory
    version: str = "latest"
    env_vars: List[str] = None
    tools: List[str] = None
    
    def __post_init__(self):
        """Initialize default values"""
        if self.env_vars is None:
            self.env_vars = []
        if self.tools is None:
            self.tools = []


class GatewayManager:
    """Manages Docker MCP Gateway operations"""
    
    def __init__(self):
        """Initialize gateway manager"""
        self.config_dir = Path.home() / ".docker" / "mcp"
        self.config_dir.mkdir(parents=True, exist_ok=True)
        self.pid_file = Path("/tmp/docker-mcp-gateway.pid")
        
    def is_gateway_running(self) -> bool:
        """Check if gateway is currently running"""
        if not self.pid_file.exists():
            return False
        
        try:
            pid = int(self.pid_file.read_text().strip())
            os.kill(pid, 0)
            return True
        except (ProcessLookupError, ValueError):
            return False
    
    def start_gateway(self, port: int, transport: str, 
                     servers: List[str], config_path: Optional[str] = None) -> bool:
        """
        Start Docker MCP Gateway
        
        Args:
            port: Port number
            transport: Transport type (sse, streaming, stdio)
            servers: List of server names to enable
            config_path: Optional config file path
            
        Returns:
            True if started successfully
        """
        try:
            cmd = [
                "docker", "mcp", "gateway", "run",
                "--port", str(port),
                "--transport", transport,
                "--verbose"
            ]
            
            if servers:
                cmd.extend(["--servers", ",".join(servers)])
            else:
                cmd.append("--enable-all-servers")
            
            if config_path:
                cmd.extend(["--config", config_path])
            
            # Start process in background
            process = subprocess.Popen(
                cmd,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                start_new_session=True
            )
            
            # Save PID
            self.pid_file.write_text(str(process.pid))
            
            return True
        except Exception as e:
            st.error(f"Failed to start gateway: {e}")
            return False
    
    def stop_gateway(self) -> bool:
        """Stop Docker MCP Gateway"""
        try:
            if self.pid_file.exists():
                pid = int(self.pid_file.read_text().strip())
                os.kill(pid, signal.SIGTERM)
                self.pid_file.unlink()
                return True
        except Exception as e:
            st.error(f"Failed to stop gateway: {e}")
        return False
    
    def save_catalog(self, servers: Dict[str, Any], filename: str = "custom-catalog.yaml"):
        """Save catalog configuration"""
        catalog_path = self.config_dir / "catalogs" / filename
        catalog_path.parent.mkdir(parents=True, exist_ok=True)
        
        catalog = {
            "version": "1.0",
            "name": "custom-catalog",
            "description": "Custom MCP server catalog",
            "servers": servers
        }
        
        with open(catalog_path, 'w') as f:
            yaml.dump(catalog, f, default_flow_style=False, sort_keys=False)
        
        return str(catalog_path)
    
    def generate_claude_config(self, host: str, port: int) -> Dict[str, Any]:
        """Generate Claude Desktop configuration"""
        return {
            "globalShortcut": "Alt+C",
            "mcpServers": {
                "docker-mcp-gateway": {
                    "command": "nc",
                    "args": [host, str(port)]
                }
            }
        }


def get_all_servers() -> List[MCPServer]:
    """Get comprehensive list of available MCP servers"""
    return [
        # Cloud & Infrastructure
        MCPServer("aws", "mcp/aws", "AWS cloud services", 
                 MCPCategory.CLOUD_INFRASTRUCTURE,
                 env_vars=["AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY"],
                 tools=["list_s3_buckets", "describe_ec2", "invoke_lambda"]),
        MCPServer("kubernetes", "mcp/kubernetes", "Kubernetes cluster management",
                 MCPCategory.CLOUD_INFRASTRUCTURE,
                 env_vars=["KUBECONFIG"],
                 tools=["get_pods", "describe_deployment"]),
        
        # Databases
        MCPServer("postgres", "mcp/postgres", "PostgreSQL database",
                 MCPCategory.DATABASES,
                 env_vars=["DATABASE_URL"],
                 tools=["execute_query", "list_tables"]),
        MCPServer("mongodb", "mcp/mongodb", "MongoDB database",
                 MCPCategory.DATABASES,
                 env_vars=["MONGODB_URI"],
                 tools=["find_documents", "insert_document"]),
        MCPServer("neo4j", "mcp/neo4j", "Neo4j graph database",
                 MCPCategory.DATABASES,
                 env_vars=["NEO4J_URI", "NEO4J_USERNAME", "NEO4J_PASSWORD"],
                 tools=["execute_cypher", "get_schema"]),
        MCPServer("elastic", "mcp/elastic", "Elasticsearch",
                 MCPCategory.DATABASES,
                 env_vars=["ELASTIC_URL"],
                 tools=["search", "index_document"]),
        MCPServer("redis", "mcp/redis", "Redis cache",
                 MCPCategory.DATABASES,
                 env_vars=["REDIS_URL"],
                 tools=["get", "set", "delete"]),
        
        # Monitoring
        MCPServer("newrelic", "mcp/newrelic", "New Relic monitoring",
                 MCPCategory.MONITORING_OBSERVABILITY,
                 env_vars=["NEW_RELIC_API_KEY"],
                 tools=["query_metrics", "get_alerts"]),
        MCPServer("grafana", "mcp/grafana", "Grafana dashboards",
                 MCPCategory.MONITORING_OBSERVABILITY,
                 env_vars=["GRAFANA_URL", "GRAFANA_API_KEY"],
                 tools=["query_datasource", "get_dashboard"]),
        MCPServer("datadog", "mcp/datadog", "Datadog monitoring",
                 MCPCategory.MONITORING_OBSERVABILITY,
                 env_vars=["DATADOG_API_KEY"],
                 tools=["query_metrics", "create_event"]),
        
        # Web Automation
        MCPServer("playwright", "mcp/playwright", "Browser automation",
                 MCPCategory.WEB_AUTOMATION,
                 tools=["navigate", "screenshot", "fill_form"]),
        MCPServer("puppeteer", "mcp/puppeteer", "Puppeteer automation",
                 MCPCategory.WEB_AUTOMATION,
                 tools=["navigate", "scrape", "pdf"]),
        MCPServer("apify", "mcp/apify", "Apify scraping platform",
                 MCPCategory.WEB_AUTOMATION,
                 env_vars=["APIFY_TOKEN"],
                 tools=["run_actor", "get_dataset"]),
        
        # Communication
        MCPServer("slack", "mcp/slack", "Slack integration",
                 MCPCategory.COMMUNICATION,
                 env_vars=["SLACK_BOT_TOKEN"],
                 tools=["send_message", "list_channels"]),
        MCPServer("discord", "mcp/discord", "Discord integration",
                 MCPCategory.COMMUNICATION,
                 env_vars=["DISCORD_TOKEN"],
                 tools=["send_message", "create_channel"]),
        MCPServer("gmail", "mcp/gmail", "Gmail operations",
                 MCPCategory.COMMUNICATION,
                 env_vars=["GMAIL_CREDENTIALS"],
                 tools=["send_email", "read_emails"]),
        
        # Payment & Finance
        MCPServer("stripe", "mcp/stripe", "Stripe payments",
                 MCPCategory.PAYMENT_FINANCE,
                 env_vars=["STRIPE_API_KEY"],
                 tools=["create_payment", "list_customers"]),
        
        # Search
        MCPServer("brave-search", "mcp/brave-search", "Brave Search",
                 MCPCategory.SEARCH_DISCOVERY,
                 env_vars=["BRAVE_API_KEY"],
                 tools=["web_search", "news_search"]),
        MCPServer("google-search", "mcp/google-search", "Google Search",
                 MCPCategory.SEARCH_DISCOVERY,
                 env_vars=["GOOGLE_API_KEY"],
                 tools=["web_search"]),
        
        # Productivity
        MCPServer("notion", "mcp/notion", "Notion workspace",
                 MCPCategory.PRODUCTIVITY,
                 env_vars=["NOTION_API_KEY"],
                 tools=["query_database", "create_page"]),
        MCPServer("jira", "mcp/jira", "Jira project management",
                 MCPCategory.PRODUCTIVITY,
                 env_vars=["JIRA_URL", "JIRA_API_TOKEN"],
                 tools=["create_issue", "search_issues"]),
        
        # Development
        MCPServer("github", "mcp/github", "GitHub integration",
                 MCPCategory.DEVELOPMENT,
                 env_vars=["GITHUB_TOKEN"],
                 tools=["search_repositories", "create_issue"]),
        MCPServer("gitlab", "mcp/gitlab", "GitLab integration",
                 MCPCategory.DEVELOPMENT,
                 env_vars=["GITLAB_TOKEN"],
                 tools=["create_merge_request", "trigger_pipeline"]),
        
        # Media
        MCPServer("youtube-transcript", "mcp/youtube-transcript", "YouTube transcripts",
                 MCPCategory.MEDIA,
                 tools=["get_transcript", "search_videos"]),
        
        # Filesystem
        MCPServer("filesystem", "mcp/filesystem", "Local filesystem",
                 MCPCategory.FILESYSTEM,
                 tools=["read_file", "write_file", "list_directory"]),
        MCPServer("google-drive", "mcp/google-drive", "Google Drive",
                 MCPCategory.FILESYSTEM,
                 env_vars=["GOOGLE_DRIVE_CREDENTIALS"],
                 tools=["list_files", "upload_file"]),
        MCPServer("s3", "mcp/s3", "AWS S3 storage",
                 MCPCategory.FILESYSTEM,
                 env_vars=["AWS_ACCESS_KEY_ID"],
                 tools=["list_objects", "upload_object"]),
    ]


def main():
    """Main Streamlit application"""
    st.title("🐳 Docker MCP Gateway Manager")
    st.markdown("Manage your Docker MCP Gateway and configure MCP servers")
    
    # Initialize session state
    if 'selected_servers' not in st.session_state:
        st.session_state.selected_servers = []
    if 'env_vars' not in st.session_state:
        st.session_state.env_vars = {}
    
    # Initialize manager
    manager = GatewayManager()
    
    # Sidebar - Gateway Status and Controls
    with st.sidebar:
        st.header("Gateway Status")
        
        is_running = manager.is_gateway_running()
        if is_running:
            st.success("🟢 Gateway Running")
        else:
            st.error("🔴 Gateway Stopped")
        
        st.divider()
        
        # Gateway Configuration
        st.header("Gateway Configuration")
        port = st.number_input("Port", min_value=1024, max_value=65535, value=8080)
        transport = st.selectbox("Transport", ["sse", "streaming", "stdio"])
        
        gateway_host = st.text_input("Gateway Host", value="localhost")
        
        st.divider()
        
        # Gateway Controls
        col1, col2 = st.columns(2)
        
        with col1:
            if st.button("▶️ Start", disabled=is_running, use_container_width=True):
                if manager.start_gateway(port, transport, st.session_state.selected_servers):
                    st.success("Gateway started!")
                    st.rerun()
        
        with col2:
            if st.button("⏹️ Stop", disabled=not is_running, use_container_width=True):
                if manager.stop_gateway():
                    st.success("Gateway stopped!")
                    st.rerun()
    
    # Main content tabs
    tab1, tab2, tab3, tab4 = st.tabs(["📦 Server Selection", "⚙️ Configuration", 
                                        "📋 Export", "📊 Summary"])
    
    # Tab 1: Server Selection
    with tab1:
        st.header("Select MCP Servers")
        
        # Get all servers
        all_servers = get_all_servers()
        
        # Group by category
        servers_by_category = {}
        for server in all_servers:
            category = server.category.value
            if category not in servers_by_category:
                servers_by_category[category] = []
            servers_by_category[category].append(server)
        
        # Category filter
        show_all_categories = st.checkbox("Show all categories", value=True)
        
        if not show_all_categories:
            selected_category = st.selectbox(
                "Select Category",
                list(servers_by_category.keys())
            )
            categories_to_show = {selected_category: servers_by_category[selected_category]}
        else:
            categories_to_show = servers_by_category
        
        # Display servers by category
        for category, servers in categories_to_show.items():
            with st.expander(f"{category} ({len(servers)} servers)", expanded=True):
                for server in servers:
                    col1, col2 = st.columns([3, 1])
                    
                    with col1:
                        is_selected = server.name in st.session_state.selected_servers
                        if st.checkbox(
                            f"**{server.name}**",
                            value=is_selected,
                            key=f"select_{server.name}"
                        ):
                            if server.name not in st.session_state.selected_servers:
                                st.session_state.selected_servers.append(server.name)
                        else:
                            if server.name in st.session_state.selected_servers:
                                st.session_state.selected_servers.remove(server.name)
                        
                        st.caption(server.description)
                        
                        if server.tools:
                            st.caption(f"🔧 Tools: {', '.join(server.tools[:3])}" + 
                                     (f" (+{len(server.tools)-3} more)" if len(server.tools) > 3 else ""))
                    
                    with col2:
                        st.caption(f"`{server.image}`")
        
        # Quick selection
        st.divider()
        col1, col2, col3 = st.columns(3)
        with col1:
            if st.button("Select All", use_container_width=True):
                st.session_state.selected_servers = [s.name for s in all_servers]
                st.rerun()
        with col2:
            if st.button("Clear All", use_container_width=True):
                st.session_state.selected_servers = []
                st.rerun()
        with col3:
            st.metric("Selected", len(st.session_state.selected_servers))
    
    # Tab 2: Configuration
    with tab2:
        st.header("Server Configuration")
        
        if not st.session_state.selected_servers:
            st.info("Select servers in the Server Selection tab to configure them")
        else:
            for server_name in st.session_state.selected_servers:
                server = next((s for s in all_servers if s.name == server_name), None)
                
                if server and server.env_vars:
                    with st.expander(f"⚙️ {server.name}", expanded=False):
                        st.markdown(f"**{server.description}**")
                        st.caption(f"Image: `{server.image}`")
                        
                        if server_name not in st.session_state.env_vars:
                            st.session_state.env_vars[server_name] = {}
                        
                        for env_var in server.env_vars:
                            value = st.text_input(
                                env_var,
                                value=st.session_state.env_vars[server_name].get(env_var, ""),
                                key=f"env_{server_name}_{env_var}",
                                type="password" if "KEY" in env_var or "SECRET" in env_var or "PASSWORD" in env_var else "default"
                            )
                            st.session_state.env_vars[server_name][env_var] = value
    
    # Tab 3: Export
    with tab3:
        st.header("Export Configuration")
        
        # Generate catalog
        if st.session_state.selected_servers:
            catalog_servers = {}
            for server_name in st.session_state.selected_servers:
                server = next((s for s in all_servers if s.name == server_name), None)
                if server:
                    server_config = {
                        "image": server.image,
                        "description": server.description,
                        "version": server.version
                    }
                    
                    if server_name in st.session_state.env_vars:
                        server_config["env"] = st.session_state.env_vars[server_name]
                    
                    catalog_servers[server_name] = server_config
            
            # Catalog YAML
            st.subheader("Catalog YAML")
            catalog_yaml = yaml.dump({
                "version": "1.0",
                "name": "custom-catalog",
                "servers": catalog_servers
            }, default_flow_style=False, sort_keys=False)
            
            st.code(catalog_yaml, language="yaml")
            st.download_button(
                "Download catalog.yaml",
                catalog_yaml,
                "docker-mcp-catalog.yaml",
                "text/yaml"
            )
            
            # Claude Desktop Config
            st.subheader("Claude Desktop Configuration")
            claude_config = manager.generate_claude_config(gateway_host, port)
            claude_config_json = json.dumps(claude_config, indent=2)
            
            st.code(claude_config_json, language="json")
            st.download_button(
                "Download claude_desktop_config.json",
                claude_config_json,
                "claude_desktop_config.json",
                "application/json"
            )
            
            # Start command
            st.subheader("Gateway Start Command")
            start_cmd = f"""docker mcp gateway run \\
    --port {port} \\
    --transport {transport} \\
    --servers "{','.join(st.session_state.selected_servers)}" \\
    --verbose"""
            st.code(start_cmd, language="bash")
        else:
            st.info("Select servers to generate configuration")
    
    # Tab 4: Summary
    with tab4:
        st.header("Configuration Summary")
        
        col1, col2 = st.columns(2)
        
        with col1:
            st.metric("Selected Servers", len(st.session_state.selected_servers))
            st.metric("Gateway Port", port)
            st.metric("Transport", transport)
        
        with col2:
            st.metric("Gateway Status", "Running" if is_running else "Stopped")
            st.metric("Configured Env Vars", len(st.session_state.env_vars))
            st.metric("Access URL", f"http://{gateway_host}:{port}")
        
        if st.session_state.selected_servers:
            st.subheader("Selected Servers")
            
            # Group selected servers by category
            selected_by_category = {}
            for server_name in st.session_state.selected_servers:
                server = next((s for s in all_servers if s.name == server_name), None)
                if server:
                    category = server.category.value
                    if category not in selected_by_category:
                        selected_by_category[category] = []
                    selected_by_category[category].append(server)
            
            for category, servers in selected_by_category.items():
                st.markdown(f"**{category}**")
                for server in servers:
                    st.markdown(f"- `{server.name}`: {server.description}")


if __name__ == "__main__":
    main()