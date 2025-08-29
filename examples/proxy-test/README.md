# Proxy Test Example

This example demonstrates that the MCP catalog commands work correctly through an HTTP proxy, testing the fix for the proxy support issue.

## What This Tests

- `docker mcp catalog show` works through HTTP proxy
- `docker mcp server list` works through HTTP proxy  
- Environment variables `HTTP_PROXY` and `HTTPS_PROXY` are properly respected

## Setup

Make sure you have coreutils installed if you're on Mac: `brew install coreutils`

The test uses:
- **Squid proxy** running on port 3128
- **Test scripts** that configure proxy environment variables
- **Docker MCP commands** to verify catalog fetching works

## Usage

### 1. Start the proxy server:
```bash
docker compose up -d
```

### 2. Run the test (with proxy):
```bash
./test-proxy.sh
```

### 3. Run without proxy (for comparison):
```bash
./test-no-proxy.sh
```

### 4. View proxy logs to see requests:
```bash
docker compose logs proxy
```

### 5. Clean up:
```bash
docker compose down
```

## Expected Results

- **With proxy**: Commands should work and you'll see HTTP requests in proxy logs
- **Without proxy**: Commands should also work (direct connection)
- **Proxy logs**: Should show requests to `desktop.docker.com`

## Troubleshooting

If tests fail:
- Check if you're behind a corporate firewall that blocks the test proxy
- Verify Docker MCP is built with the proxy fix
- Check proxy logs for errors: `docker compose logs proxy`