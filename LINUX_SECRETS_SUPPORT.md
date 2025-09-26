# Linux Secrets Support for MCP Gateway

This document describes the enhanced secrets support for Linux environments without Docker Desktop.

## Problem

Previously, the MCP Gateway's secret management commands (`docker mcp secret set`, `docker mcp secret ls`, `docker mcp secret rm`) required Docker Desktop to be installed and running. In Linux environments without Docker Desktop, these commands would fail with an error like:

```
Get "http://localhost/secrets": dial unix /home/user/.docker/desktop/jfs.sock: connect: no such file or directory
```

## Solution

We've implemented a comprehensive fallback system that automatically detects the available secret storage backends and uses them in the following priority order:

1. **Docker Desktop** (if available)
2. **Docker Credential Store** (if `docker-credential-pass` is available)
3. **Local File Storage** (always available as last resort)

## Secret Storage Backends

### 1. Docker Desktop Provider
- **When used**: When Docker Desktop is installed and the JFS socket is available
- **Storage**: Uses Docker Desktop's internal secret storage
- **Security**: Managed by Docker Desktop

### 2. Credential Store Provider
- **When used**: When `docker-credential-pass` is available on the system
- **Storage**: Uses Docker's credential helper system (usually GPG-encrypted)
- **Security**: Relies on system GPG setup and the credential helper

### 3. File Provider (Fallback)
- **When used**: Always available as the last resort
- **Storage**: `~/.docker/mcp/secrets/secrets.json`
- **Security**: AES-256-GCM encrypted with a local key stored in `~/.docker/mcp/secrets/.key`
- **Permissions**: Files are created with 600 permissions (readable only by the user)

## Usage

The secret commands work exactly the same way as before, but now automatically fall back to available storage methods:

```bash
# Set a secret
docker mcp secret set api_key=your_secret_value

# List secrets  
docker mcp secret ls

# Remove a secret
docker mcp secret rm api_key

# Remove all secrets
docker mcp secret rm --all
```

## Installation on Ubuntu

No additional installation is required. The fallback system is built into the MCP Gateway and will work out of the box on any Linux system.

For enhanced security with credential store support, you can optionally install:

```bash
# Install docker-credential-pass for credential store support
sudo apt-get install pass docker-credential-pass

# Set up GPG if not already configured
gpg --generate-key
pass init your-gpg-key-id
```

## Security Considerations

### File Provider Security
- Secrets are encrypted using AES-256-GCM
- Each secret has a unique nonce
- The encryption key is stored locally with 600 permissions
- The secrets directory is created with 700 permissions

### Best Practices
1. **Use Docker Desktop** when available for the most secure option
2. **Set up GPG and credential store** for better security than file storage
3. **Protect your home directory** since the fallback stores secrets there
4. **Regular backups** of your secret storage if using file provider

## Backward Compatibility

This change is fully backward compatible:
- Existing Docker Desktop users will see no change in behavior
- Gateway configuration with `--secrets=docker-desktop:/.env` continues to work
- All existing documentation and examples remain valid

## Implementation Details

The system uses a provider chain pattern:
- Each provider implements a common `SecretProvider` interface
- The chain tries providers in order until one succeeds
- Operations automatically fall back to the next available provider
- List operations aggregate results from all available providers

## Troubleshooting

### No secrets listed but you know they exist
- Check if you're running from the same user account that created the secrets
- Verify permissions on `~/.docker/mcp/secrets/`

### Permission denied errors
```bash
# Fix permissions if needed
chmod 700 ~/.docker/mcp/secrets/
chmod 600 ~/.docker/mcp/secrets/*
```

### Docker Desktop not detected
The system will automatically detect if Docker Desktop is unavailable and fall back to other methods. This is expected behavior on standard Linux installations.

## Migration from Docker Desktop

If you're moving from a Docker Desktop environment to a Linux environment:

1. **Export your secrets** from Docker Desktop using `docker mcp secret ls --json`
2. **Recreate them** on the new system using `docker mcp secret set`
3. The new system will automatically use the best available storage method

## Testing

The implementation includes comprehensive tests:

```bash
# Run provider tests
go test ./cmd/docker-mcp/secret-management/provider -v

# Test the secret commands
docker mcp secret set test=value
docker mcp secret ls
docker mcp secret rm test
```