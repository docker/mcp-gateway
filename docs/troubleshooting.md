# Troubleshooting the MCP Gateway

Sometimes, you plug the MCP Gateway into your favorite MCP client and it doesn't work as expected.
What can you do to pinpoint where the problem comes from?

## Debug the MCP Gateway's startup sequence

The go to command to start a fresh Gateway, plugged into Docker Desltop's Toolkit is this one:

```console
docker mcp gateway run --profile my-profile --verbose --dry-run
```

This will show you how the Gateway is reading the configuration, which servers are actually
enabled, which images are pulled and which MCP servers are started with which command line arguments.

It'll show you how many tools you have, in aggregate.

This is usually a good way to troubleshoot missing images, invalid server names, missing config
or secrets...

## Debug tool calls

Full fledge MCP clients might sometimes hide the errors they encounter while calling tools
on the MCP Gateway.

Here's a useful set of commands you can use to debug your tool calls:

```console
# List the aggregate number of tools
docker mcp tools ls

# To see what's going on in the gateway while listing tools
docker mcp tools ls --verbose

# Call one of the tools
docker mcp tools call search query=Docker

# Be verbose and pass additional parameters to the Gateway
docker mcp tools call --gateway-arg="--servers=duckduckgo" --verbose search query=Docker
```
