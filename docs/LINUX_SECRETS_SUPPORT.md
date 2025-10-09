# Linux Secrets Support

This document explains how the MCP Gateway manages secrets on Linux systems, the available storage strategies, and how to choose and configure the right option for your environment.

## Overview

The MCP Gateway provides secure secret management for MCP servers through multiple storage backends. On Linux, the available strategy depends on your Docker environment configuration.

Use `docker mcp secret diagnose` to check which secret storage strategies are available in your environment.

## Storage Strategies

### 1. Docker Swarm Mode (Recommended for Production)

Docker Swarm provides native encrypted secret storage and distribution. This is the most secure option for production environments.

**Features:**
- Encrypted storage at rest
- Encrypted distribution to containers
- TLS mutual authentication
- Secrets stored in Raft consensus log
- Fine-grained access control

**Setup:**

```bash
# Initialize Swarm mode
docker swarm init

# Verify Swarm is active
docker info | grep "Swarm: active"

# Run diagnostics to confirm
docker mcp secret diagnose
```

**Usage:**

```bash
# Set a secret
docker mcp secret set API_KEY=your-secret-value

# List secrets
docker mcp secret ls

# Remove a secret
docker mcp secret rm API_KEY
```

**Security Notes:**
- Secrets are encrypted using AES-256-GCM
- Only containers with explicit access can read secrets
- Secrets are mounted in-memory (tmpfs) in containers
- Never written to disk in plaintext

### 2. Docker Desktop

Docker Desktop provides integrated credential storage through the operating system's native keychain.

**Features:**
- OS-native secure storage
- Integration with system credential helpers
- Encrypted storage
- GUI integration

**Setup:**

Install Docker Desktop for Linux following the [official documentation](https://docs.docker.com/desktop/install/linux-install/).

**Availability:**
Currently, Docker Desktop on Linux is available for:
- Ubuntu
- Debian
- Fedora
- Arch-based distributions

### 3. File-Based Storage (Development Only)

⚠️ **WARNING: Not suitable for production use**

File-based storage saves secrets as encrypted files in the local filesystem. This is a fallback option when neither Swarm nor Docker Desktop is available.

**Features:**
- Simple setup
- No external dependencies
- Encrypted at rest (AES-256-GCM)
- Suitable for development and testing

**Setup:**

No additional setup required. This is the automatic fallback when other options are unavailable.

**Storage Location:**

```bash
~/.docker/mcp/secrets/
```

**Security Limitations:**
- Secrets stored on local filesystem
- Protection depends on filesystem permissions
- Encryption key stored locally
- Not suitable for multi-host deployments
- No built-in rotation mechanism

**Usage:**

```bash
# Set a secret
docker mcp secret set API_KEY=your-secret-value

# Secrets are stored in ~/.docker/mcp/secrets/
ls -la ~/.docker/mcp/secrets/

# List secrets
docker mcp secret ls

# Remove a secret
docker mcp secret rm API_KEY
```

## Choosing the Right Strategy

| Use Case | Recommended Strategy | Why |
|:---------|:-------------------|:----|
| Production deployment | Docker Swarm | Encrypted storage, distributed secrets, access control |
| Development on desktop | Docker Desktop | OS-native security, easy setup |
| Development on server | Docker Swarm (init) | Better security than file-based |
| CI/CD pipelines | Docker Swarm | Automated secret distribution |
| Local testing only | File-based | Simple, no setup required |

## Migration Between Strategies

### From File to Swarm

```bash
# 1. List current secrets
docker mcp secret ls

# 2. Initialize Swarm
docker swarm init

# 3. Secrets are automatically migrated to Swarm storage
docker mcp secret diagnose
```

### From Swarm to File

```bash
# 1. Leave Swarm mode (WARNING: This will delete Swarm secrets)
docker swarm leave --force

# 2. Recreate secrets using file storage
docker mcp secret set API_KEY=value
```

## Security Best Practices

### General Guidelines

1. **Never commit secrets to version control**
   - Add `.docker/mcp/secrets/` to `.gitignore`
   - Use environment-specific secret management

2. **Use appropriate strategy for environment**
   - Production: Docker Swarm or external secret manager
   - Development: Docker Desktop or Swarm
   - Testing: File-based acceptable

3. **Rotate secrets regularly**
   ```bash
   # Update a secret
   docker mcp secret set API_KEY=new-value
   ```

4. **Limit secret access**
   - Only provide secrets to containers that need them
   - Use label-based secret injection: `-l x-secret:KEY=/path/to/secret`

5. **Audit secret usage**
   ```bash
   # Review which secrets exist
   docker mcp secret ls

   # Check diagnostics
   docker mcp secret diagnose
   ```

### File-Based Storage Security

If using file-based storage:

```bash
# Ensure proper permissions
chmod 700 ~/.docker/mcp/secrets/
chmod 600 ~/.docker/mcp/secrets/*

# Regular backup (encrypted)
tar czf secrets-backup.tar.gz ~/.docker/mcp/secrets/
gpg -c secrets-backup.tar.gz
rm secrets-backup.tar.gz
```

### Swarm Mode Security

```bash
# Use external CA for additional security
docker swarm init --external-ca

# Rotate join tokens regularly
docker swarm join-token --rotate worker
docker swarm join-token --rotate manager

# Autolock Swarm for encryption at rest
docker swarm init --autolock
```

## Environment-Specific Configuration

### Using Secrets in Containers

Secrets are injected into containers using labels:

```bash
# Method 1: Mount as file
docker run -d \
  -l x-secret:API_KEY=/run/secrets/api_key \
  -e API_KEY_FILE=/run/secrets/api_key \
  your-mcp-server

# Method 2: Set as environment variable (less secure)
docker run -d \
  -l x-secret:API_KEY=env:API_KEY \
  your-mcp-server
```

### Secrets with Docker Compose

```yaml
services:
  mcp-server:
    image: your-mcp-server
    labels:
      - x-secret:API_KEY=/run/secrets/api_key
    environment:
      - API_KEY_FILE=/run/secrets/api_key
```

## Troubleshooting

### Diagnostics Command

```bash
docker mcp secret diagnose
```

**Output interpretation:**

```
Docker Desktop:       ❌ No     - Docker Desktop not installed
Swarm Mode:           ✅ Yes    - Swarm initialized
Credential Helper:    ✅ Yes    - System credential helper available
Secure Mount Support: ✅ Yes    - Secrets can be mounted securely
Recommended Strategy: swarm     - Using Swarm storage
```

### Common Issues

#### Secret not found

```bash
# Check if secret exists
docker mcp secret ls

# Recreate if missing
docker mcp secret set SECRET_NAME=value
```

#### Permission denied

```bash
# File-based: Check directory permissions
ls -la ~/.docker/mcp/secrets/

# Fix permissions
chmod 700 ~/.docker/mcp/secrets/
```

#### Swarm mode issues

```bash
# Check Swarm status
docker info | grep Swarm

# Reinitialize if needed
docker swarm leave --force
docker swarm init
```

#### Secret not injected into container

```bash
# Verify label syntax
docker inspect container-name | grep -A 5 Labels

# Correct label format
-l x-secret:SECRET_NAME=/path/in/container

# Check secret exists
docker mcp secret ls | grep SECRET_NAME
```

### Debugging Secret Injection

```bash
# Enable debug logging
export MCP_DEBUG=1

# Run container and check logs
docker logs container-name

# Verify secret mount inside container
docker exec container-name ls -la /run/secrets/
```

## Command Reference

For detailed information about each command, see:

- [`docker mcp secret`](generator/reference/mcp_secret.md) - Overview of secret management commands
- [`docker mcp secret set`](generator/reference/mcp_secret_set.md) - Set a secret
- [`docker mcp secret ls`](generator/reference/mcp_secret_ls.md) - List secrets
- [`docker mcp secret rm`](generator/reference/mcp_secret_rm.md) - Remove secrets

## Examples

### Example 1: PostgreSQL Password

```bash
# Store PostgreSQL password
docker mcp secret set POSTGRES_PASSWORD=my-secure-password

# Run PostgreSQL with secret
docker run -d \
  -l x-secret:POSTGRES_PASSWORD=/run/secrets/postgres_pwd \
  -e POSTGRES_PASSWORD_FILE=/run/secrets/postgres_pwd \
  -p 5432:5432 \
  postgres
```

### Example 2: API Keys for MCP Server

```bash
# Store multiple API keys
docker mcp secret set OPENAI_API_KEY=sk-...
docker mcp secret set ANTHROPIC_API_KEY=sk-ant-...

# Run MCP server with both secrets
docker run -d \
  -l x-secret:OPENAI_API_KEY=/secrets/openai \
  -l x-secret:ANTHROPIC_API_KEY=/secrets/anthropic \
  -e OPENAI_API_KEY_FILE=/secrets/openai \
  -e ANTHROPIC_API_KEY_FILE=/secrets/anthropic \
  your-mcp-server
```

### Example 3: Reading Secret from STDIN

```bash
# Generate and store a random password
openssl rand -base64 32 | docker mcp secret set DB_PASSWORD

# Or from a file
cat /path/to/secret.txt | docker mcp secret set MY_SECRET
```

### Example 4: Multiple Environment Setup

```bash
# Development secrets
docker mcp secret set DEV_API_KEY=dev-key-123

# Production secrets (on production host with Swarm)
docker mcp secret set PROD_API_KEY=prod-key-456

# List to verify
docker mcp secret ls
```

## External Secret Management Integration

For enterprise deployments, consider integrating with external secret managers:

- **HashiCorp Vault**: Use Vault as secrets backend
- **AWS Secrets Manager**: For AWS-deployed MCP Gateways
- **Azure Key Vault**: For Azure environments
- **GCP Secret Manager**: For Google Cloud deployments

These integrations require custom provider implementations. See the `--provider` flag documentation in [`docker mcp secret set`](generator/reference/mcp_secret_set.md).

## See Also

- [Security Documentation](security.md) - Overall security architecture
- [Troubleshooting Guide](troubleshooting.md) - General troubleshooting
- [Docker Secrets Documentation](https://docs.docker.com/engine/swarm/secrets/) - Official Docker secrets docs
