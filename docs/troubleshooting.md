# Troubleshooting the MCP Gateway

Sometimes, you plug the MCP Gateway into your favorite MCP client and it doesn't work as expected.
What can you do to pinpoint where the problem comes from?

## Debug the MCP Gateway's startup sequence

The go to command to start a fresh Gateway, plugged into Docker Desltop's Toolkit is this one:

```console
docker mcp gateway run --verbose --dry-run
```

This will show you how the Gateway is reading the configuration, which servers are actually
enabled, which images are pulled and which MCP servers are started with which command line arguments.

It'll show you how many tools you have, in aggregate.

This is usually a good way to troubleshoot missing images, invalid server names, missing config
or secrets...

You can also focus a one given server with:

```console
docker mcp gateway run --verbose --dry-run --servers=duckduckgo
```

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

## A client rejects every tool of one server: "unsupported dialect"

Some clients validate tool schemas with a validator configured for JSON Schema 2020-12
only, and refuse a tool whose schema declares an older dialect. The error names the tool
but applies to every tool on that server:

```
Tool 'list_bases' has an invalid outputSchema: JSON Schema declares an unsupported
dialect ("$schema": "http://json-schema.org/draft-07/schema#").
```

This is not a broken or stale server image. MCP lets a schema declare any dialect and only
requires clients to support 2020-12, and servers built on the MCP TypeScript SDK declare
draft-07 for every tool because its zod converter defaults to that target.

The gateway translates such schemas into 2020-12 before advertising them, so an up to date
gateway resolves this on its own. To see what a server actually declared, compare:

```console
# what clients are served
docker mcp tools ls --format json | jq '.[] | {name, dialect: .inputSchema["$schema"]}'

# what the server declared, with translation turned off
docker mcp tools ls --gateway-arg="--preserve-tool-schema-dialect" --format json | jq '.[] | {name, dialect: .inputSchema["$schema"]}'
```

A schema the gateway cannot translate without changing what it accepts is relayed as the
server declared it, and the reason is logged under `--verbose`:

```
> Relaying outputSchema of tool "x" from some-server with its declared dialect: ...
```
