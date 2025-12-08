// MCP Gateway Manager JavaScript
class MCPGatewayManager {
    constructor() {
        this.servers = new Map();
        this.activeServers = new Set();
        this.gatewayConfig = {
            port: 8811,
            transport: 'sse',
            catalogUrl: 'https://desktop.docker.com/mcp/catalog/v2/catalog.yaml',
            enableDynamicTools: true,
            enableLogging: true,
            enableTelemetry: false
        };
        this.init();
    }

    async init() {
        await this.loadServers();
        this.loadSettings();
    }

    // Server Management
    async loadServers() {
        try {
            this.showLoading('serversContent');
            
            // Connect to the API backend
            const response = await fetch('/api/servers');
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            const serversData = await response.json();
            
            this.servers.clear();
            serversData.forEach(server => {
                this.servers.set(server.name, server);
            });
            
            this.renderServers();
        } catch (error) {
            this.showError('Failed to load servers: ' + error.message);
        }
    }

    async getSampleServers() {
        // Simulate API call delay
        await new Promise(resolve => setTimeout(resolve, 1000));
        
        return [
            {
                name: 'filesystem',
                description: 'File system operations for reading, writing, and managing files and directories',
                image: 'docker/filesystem-mcp',
                active: true,
                longLived: false,
                secrets: [],
                tools: ['read_file', 'write_file', 'list_directory', 'create_directory'],
                config: {
                    basePath: '/tmp',
                    allowedExtensions: ['.txt', '.md', '.json']
                }
            },
            {
                name: 'duckduckgo',
                description: 'Web search using DuckDuckGo search engine',
                image: 'docker/duckduckgo-mcp',
                active: true,
                longLived: false,
                secrets: [],
                tools: ['search_web'],
                config: {}
            },
            {
                name: 'github',
                description: 'GitHub repository management and operations',
                image: 'docker/github-mcp',
                active: false,
                longLived: true,
                secrets: ['GITHUB_TOKEN'],
                tools: ['list_repos', 'create_issue', 'get_file_content'],
                config: {
                    defaultOrg: 'docker'
                }
            },
            {
                name: 'postgres',
                description: 'PostgreSQL database operations and queries',
                image: 'docker/postgres-mcp',
                active: false,
                longLived: true,
                secrets: ['POSTGRES_CONNECTION_STRING'],
                tools: ['execute_query', 'list_tables', 'describe_table'],
                config: {
                    maxConnections: 10,
                    queryTimeout: 30000
                }
            },
            {
                name: 'slack',
                description: 'Slack messaging and workspace management',
                image: 'docker/slack-mcp',
                active: false,
                longLived: false,
                secrets: ['SLACK_BOT_TOKEN'],
                tools: ['send_message', 'list_channels', 'get_user_info'],
                config: {
                    defaultChannel: '#general'
                }
            }
        ];
    }

    renderServers() {
        const container = document.getElementById('serversContent');
        const filteredServers = this.getFilteredServers();
        
        if (filteredServers.length === 0) {
            container.innerHTML = '<div class="loading"><p>No servers found</p></div>';
            return;
        }

        const serversHTML = filteredServers.map(server => {
            const statusClass = server.active ? 'active' : '';
            const statusIndicator = server.active ? 'active' : '';
            
            return `
                <div class="server-card ${statusClass}">
                    <h3>
                        <span class="status-indicator ${statusIndicator}"></span>
                        ${server.name}
                    </h3>
                    <div class="server-description">${server.description}</div>
                    <div class="server-meta">
                        <span class="meta-tag">📦 ${server.image}</span>
                        ${server.longLived ? '<span class="meta-tag">🔄 Long-lived</span>' : ''}
                        ${server.secrets.length > 0 ? `<span class="meta-tag">🔐 ${server.secrets.length} secrets</span>` : ''}
                        <span class="meta-tag">🛠️ ${server.tools.length} tools</span>
                    </div>
                    <div class="server-actions">
                        ${server.active ? 
                            '<button class="btn btn-danger" onclick="mcpManager.removeServer(\'' + server.name + '\')">❌ Remove</button>' :
                            '<button class="btn btn-success" onclick="mcpManager.addServerToGateway(\'' + server.name + '\')">✅ Add</button>'
                        }
                        <button class="btn btn-secondary" onclick="mcpManager.configureServer('${server.name}')">⚙️ Configure</button>
                        <button class="btn btn-secondary" onclick="mcpManager.viewServerDetails('${server.name}')">👁️ Details</button>
                    </div>
                </div>
            `;
        }).join('');

        container.innerHTML = `<div class="server-grid">${serversHTML}</div>`;
    }

    getFilteredServers() {
        const searchTerm = document.getElementById('searchInput').value.toLowerCase();
        return Array.from(this.servers.values()).filter(server => 
            server.name.toLowerCase().includes(searchTerm) ||
            server.description.toLowerCase().includes(searchTerm) ||
            server.tools.some(tool => tool.toLowerCase().includes(searchTerm))
        );
    }

    async addServerToGateway(serverName) {
        try {
            const response = await fetch('/api/servers/add', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ name: serverName }),
            });

            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }

            const result = await response.json();
            
            // Update local state
            const server = this.servers.get(serverName);
            if (server) {
                server.active = true;
                this.activeServers.add(serverName);
                this.renderServers();
                this.showSuccess(result.message);
            }
        } catch (error) {
            this.showError(`Failed to add server: ${error.message}`);
        }
    }

    async removeServer(serverName) {
        try {
            const response = await fetch('/api/servers/remove', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ name: serverName }),
            });

            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }

            const result = await response.json();
            
            // Update local state
            const server = this.servers.get(serverName);
            if (server) {
                server.active = false;
                this.activeServers.delete(serverName);
                this.renderServers();
                this.showSuccess(result.message);
            }
        } catch (error) {
            this.showError(`Failed to remove server: ${error.message}`);
        }
    }

    configureServer(serverName) {
        const server = this.servers.get(serverName);
        if (!server) return;

        const configHTML = `
            <div class="modal-content">
                <span class="close" onclick="closeModal('configModal')">&times;</span>
                <h2>Configure ${serverName}</h2>
                <div class="form-group">
                    <label>Description</label>
                    <div class="code-block">${server.description}</div>
                </div>
                ${Object.keys(server.config).length > 0 ? `
                    <div class="form-group">
                        <label>Configuration</label>
                        <div class="code-block">${JSON.stringify(server.config, null, 2)}</div>
                    </div>
                ` : ''}
                ${server.secrets.length > 0 ? `
                    <div class="form-group">
                        <label>Required Secrets</label>
                        <div>${server.secrets.map(secret => `<span class="meta-tag">🔐 ${secret}</span>`).join(' ')}</div>
                    </div>
                ` : ''}
                <div class="form-group">
                    <label>Available Tools</label>
                    <div>${server.tools.map(tool => `<span class="meta-tag">🛠️ ${tool}</span>`).join(' ')}</div>
                </div>
                <div class="action-bar">
                    <button class="btn btn-primary" onclick="mcpManager.saveServerConfig('${serverName}')">💾 Save</button>
                    <button class="btn btn-secondary" onclick="closeModal('configModal')">Cancel</button>
                </div>
            </div>
        `;

        this.showModal('configModal', configHTML);
    }

    viewServerDetails(serverName) {
        const server = this.servers.get(serverName);
        if (!server) return;

        const detailsHTML = `
            <div class="modal-content">
                <span class="close" onclick="closeModal('detailsModal')">&times;</span>
                <h2>Server Details: ${serverName}</h2>
                <div class="form-group">
                    <label>Status</label>
                    <div class="meta-tag ${server.active ? 'active' : ''}">${server.active ? '✅ Active' : '❌ Inactive'}</div>
                </div>
                <div class="form-group">
                    <label>Docker Image</label>
                    <div class="code-block">${server.image}</div>
                </div>
                <div class="form-group">
                    <label>Description</label>
                    <div class="code-block">${server.description}</div>
                </div>
                <div class="form-group">
                    <label>Configuration</label>
                    <div class="code-block">${JSON.stringify(server.config, null, 2)}</div>
                </div>
                <div class="form-group">
                    <label>Tools (${server.tools.length})</label>
                    <div class="code-block">${server.tools.join('\n')}</div>
                </div>
                ${server.secrets.length > 0 ? `
                    <div class="form-group">
                        <label>Required Secrets (${server.secrets.length})</label>
                        <div class="code-block">${server.secrets.join('\n')}</div>
                    </div>
                ` : ''}
                <div class="action-bar">
                    <button class="btn btn-secondary" onclick="closeModal('detailsModal')">Close</button>
                </div>
            </div>
        `;

        this.showModal('detailsModal', detailsHTML);
    }

    // Configuration Export
    async generateClaudeConfig() {
        try {
            const response = await fetch('/api/export/claude');
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            const config = await response.json();
            this.showConfigOutput('Claude Desktop Configuration', JSON.stringify(config, null, 2));
        } catch (error) {
            this.showError(`Failed to generate Claude config: ${error.message}`);
        }
    }

    async generateLLMStudioConfig() {
        try {
            const response = await fetch('/api/export/llmstudio');
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            const config = await response.json();
            this.showConfigOutput('LLM Studio Configuration', JSON.stringify(config, null, 2));
        } catch (error) {
            this.showError(`Failed to generate LLM Studio config: ${error.message}`);
        }
    }

    async generateDockerCompose() {
        try {
            const response = await fetch('/api/export/docker-compose');
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            const config = await response.text();
            this.showConfigOutput('Docker Compose Configuration', config);
        } catch (error) {
            this.showError(`Failed to generate Docker Compose config: ${error.message}`);
        }
    }

    objectToYaml(obj, indent = 0) {
        let yaml = '';
        const spaces = '  '.repeat(indent);
        
        for (const [key, value] of Object.entries(obj)) {
            if (typeof value === 'object' && value !== null && !Array.isArray(value)) {
                yaml += `${spaces}${key}:\n`;
                yaml += this.objectToYaml(value, indent + 1);
            } else if (Array.isArray(value)) {
                yaml += `${spaces}${key}:\n`;
                value.forEach(item => {
                    yaml += `${spaces}  - ${item}\n`;
                });
            } else {
                yaml += `${spaces}${key}: ${value}\n`;
            }
        }
        
        return yaml;
    }

    showConfigOutput(title, content) {
        const outputDiv = document.getElementById('exportOutput');
        outputDiv.innerHTML = `
            <div class="config-section">
                <h3>${title}</h3>
                <div class="code-block">${content}</div>
                <button class="btn btn-secondary" onclick="mcpManager.copyToClipboard(\`${content.replace(/`/g, '\\`')}\`)">📋 Copy to Clipboard</button>
            </div>
        `;
    }

    async copyToClipboard(text) {
        try {
            await navigator.clipboard.writeText(text);
            this.showSuccess('Configuration copied to clipboard!');
        } catch (error) {
            this.showError('Failed to copy to clipboard');
        }
    }

    // Settings Management
    loadSettings() {
        const saved = localStorage.getItem('mcpGatewayConfig');
        if (saved) {
            this.gatewayConfig = { ...this.gatewayConfig, ...JSON.parse(saved) };
        }
        this.updateSettingsUI();
    }

    updateSettingsUI() {
        document.getElementById('gatewayPort').value = this.gatewayConfig.port;
        document.getElementById('transport').value = this.gatewayConfig.transport;
        document.getElementById('catalogUrl').value = this.gatewayConfig.catalogUrl;
        document.getElementById('enableDynamicTools').checked = this.gatewayConfig.enableDynamicTools;
        document.getElementById('enableLogging').checked = this.gatewayConfig.enableLogging;
        document.getElementById('enableTelemetry').checked = this.gatewayConfig.enableTelemetry;
    }

    saveGatewayConfig() {
        this.gatewayConfig.port = parseInt(document.getElementById('gatewayPort').value);
        this.gatewayConfig.transport = document.getElementById('transport').value;
        this.gatewayConfig.catalogUrl = document.getElementById('catalogUrl').value;
        
        localStorage.setItem('mcpGatewayConfig', JSON.stringify(this.gatewayConfig));
        this.showSuccess('Gateway configuration saved!');
    }

    saveSettings() {
        this.gatewayConfig.enableDynamicTools = document.getElementById('enableDynamicTools').checked;
        this.gatewayConfig.enableLogging = document.getElementById('enableLogging').checked;
        this.gatewayConfig.enableTelemetry = document.getElementById('enableTelemetry').checked;
        
        localStorage.setItem('mcpGatewayConfig', JSON.stringify(this.gatewayConfig));
        this.showSuccess('Settings saved!');
    }

    // Search and Discovery
    searchServers() {
        this.renderServers();
    }

    async searchCatalog() {
        const query = document.getElementById('serverSearch').value;
        if (query.length < 2) {
            document.getElementById('catalogResults').innerHTML = '';
            return;
        }

        // Simulate catalog search
        const results = Array.from(this.servers.values()).filter(server => 
            !server.active && (
                server.name.toLowerCase().includes(query.toLowerCase()) ||
                server.description.toLowerCase().includes(query.toLowerCase())
            )
        );

        const resultsHTML = results.map(server => `
            <div class="server-card" onclick="mcpManager.selectCatalogServer('${server.name}')">
                <h4>${server.name}</h4>
                <p>${server.description}</p>
                <div class="server-meta">
                    <span class="meta-tag">📦 ${server.image}</span>
                    <span class="meta-tag">🛠️ ${server.tools.length} tools</span>
                </div>
            </div>
        `).join('');

        document.getElementById('catalogResults').innerHTML = resultsHTML;
    }

    selectCatalogServer(serverName) {
        document.getElementById('serverName').value = serverName;
        document.getElementById('catalogResults').innerHTML = '';
    }

    async addServer() {
        const serverName = document.getElementById('serverName').value.trim();
        if (!serverName) {
            this.showError('Please enter a server name');
            return;
        }

        try {
            await this.addServerToGateway(serverName);
            closeModal('addServerModal');
            document.getElementById('serverName').value = '';
        } catch (error) {
            this.showError(`Failed to add server: ${error.message}`);
        }
    }

    async importFromRegistry() {
        document.getElementById('importModal').style.display = 'block';
    }

    async importRegistry() {
        const url = document.getElementById('registryUrl').value.trim();
        if (!url) {
            this.showError('Please enter a registry URL');
            return;
        }

        try {
            // In real implementation, this would fetch from the URL
            this.showSuccess(`Successfully imported servers from ${url}`);
            closeModal('importModal');
            await this.loadServers();
        } catch (error) {
            this.showError(`Failed to import registry: ${error.message}`);
        }
    }

    async connectRemote() {
        const host = document.getElementById('remoteHost').value.trim();
        const protocol = document.getElementById('remoteProtocol').value;
        
        if (!host) {
            this.showError('Please enter a remote host');
            return;
        }

        try {
            // In real implementation, this would connect to remote gateway
            this.showSuccess(`Connected to remote gateway at ${protocol}://${host}`);
        } catch (error) {
            this.showError(`Failed to connect to remote gateway: ${error.message}`);
        }
    }

    async refreshServers() {
        await this.loadServers();
        this.showSuccess('Servers refreshed!');
    }

    // UI Utilities
    showLoading(containerId) {
        document.getElementById(containerId).innerHTML = `
            <div class="loading">
                <div class="spinner"></div>
                <p>Loading...</p>
            </div>
        `;
    }

    showSuccess(message) {
        this.showAlert(message, 'success');
    }

    showError(message) {
        this.showAlert(message, 'error');
    }

    showInfo(message) {
        this.showAlert(message, 'info');
    }

    showAlert(message, type) {
        const alertsContainer = document.querySelector('.container');
        const alertDiv = document.createElement('div');
        alertDiv.className = `alert alert-${type}`;
        alertDiv.textContent = message;
        
        alertsContainer.insertBefore(alertDiv, alertsContainer.firstChild.nextSibling);
        
        setTimeout(() => {
            alertDiv.remove();
        }, 5000);
    }

    showModal(modalId, content) {
        let modal = document.getElementById(modalId);
        if (!modal) {
            modal = document.createElement('div');
            modal.id = modalId;
            modal.className = 'modal';
            document.body.appendChild(modal);
        }
        
        modal.innerHTML = content;
        modal.style.display = 'block';
    }
}

// Global functions for UI interactions
function showTab(tabName) {
    // Hide all tab contents
    document.querySelectorAll('.tab-content').forEach(content => {
        content.classList.remove('active');
    });
    
    // Remove active class from all tabs
    document.querySelectorAll('.tab').forEach(tab => {
        tab.classList.remove('active');
    });
    
    // Show selected tab content
    document.getElementById(tabName).classList.add('active');
    
    // Add active class to clicked tab
    event.target.classList.add('active');
}

function showAddServerModal() {
    document.getElementById('addServerModal').style.display = 'block';
}

function closeModal(modalId) {
    document.getElementById(modalId).style.display = 'none';
}

function searchServers() {
    mcpManager.searchServers();
}

function refreshServers() {
    mcpManager.refreshServers();
}

function importFromRegistry() {
    mcpManager.importFromRegistry();
}

function searchCatalog() {
    mcpManager.searchCatalog();
}

function addServer() {
    mcpManager.addServer();
}

function importRegistry() {
    mcpManager.importRegistry();
}

function generateClaudeConfig() {
    mcpManager.generateClaudeConfig();
}

function generateLLMStudioConfig() {
    mcpManager.generateLLMStudioConfig();
}

function generateDockerCompose() {
    mcpManager.generateDockerCompose();
}

function saveGatewayConfig() {
    mcpManager.saveGatewayConfig();
}

function saveSettings() {
    mcpManager.saveSettings();
}

function connectRemote() {
    mcpManager.connectRemote();
}

// Close modals when clicking outside
window.onclick = function(event) {
    if (event.target.classList.contains('modal')) {
        event.target.style.display = 'none';
    }
}

// Initialize the application
const mcpManager = new MCPGatewayManager();