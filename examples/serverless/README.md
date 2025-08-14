# Serverless MCP Servers

This example demonstrates how to use the MCP Gateway with Serverless-deployed MCP servers that communicate via SSE.

## Overview

The MCP Gateway can deploy MCP server compositions to Serverless platforms using `kubectl apply` and then connect to them via SSE endpoints. 

## Configuration

Configure a Serverless MCP server in your catalog.yaml:

```yaml
registry:
  my-serverless-server:
    serverless:
      configPath: ./path/to/deployment.yaml  # Path to Serverless YAML config, see below for how to generate for simple-agent-mcp
      namespace: default                     # Optional, defaults to "default"
    remote:
      url: "http://my-server.local/sse"     # SSE endpoint where server will be reachable
      transport_type: sse                    # Must be "sse" or "http"
    longLived: true                         # Recommended for serverless servers
```

There is an exmaple catalog in this directory that has been tested against EKS playground with the `serverless/examples/simple-agent` composition `simple-agent-mcp`. You can use make in `examples/simple-agent` directory to generate the composition deployment yaml. (After also make-ing and pushing those images to ECR.)

You then need to port-forward to localhost:9999 (matching `catalog.yaml` in this directory), which can be done with the eks-playground's `make jump-forward` to port forward through your jump box to the eks cluster.

After building the mcp gateway, and installing it in docker (the below build target does both)

```
make docker-mcp
```

you can run the gateway

```
docker mcp gateway run --servers=remote-serverless --transport sse --catalog=examples/serverless/catalog.yaml
```

And for example install the gateway as an mcp server in claude code:

```
> claude mcp add --transport sse mcp-gateway http://localhost:8811/sse
```

within claude code, you should be able to:

```
> ask the mcp-gateway chat tool "where is iceland"

  mcp-gateway - chat (MCP)(message: "where is iceland")
  ⎿ Iceland is a Nordic island country located in the North Atlantic Ocean. It is situated between Greenland and mainland Europe. To the east of Iceland is the Faroe Islands, and to 
    the southeast is the United Kingdom. It lies just south of the Arctic Circle, making it one of the northernmost countries in the world. Despite its name, Iceland is known for its
     relatively mild climate compared to other locations at similar latitudes, thanks to the influence of the North Atlantic Current. Its capital and largest city is Reykjavík.


⏺ Iceland is a Nordic island country located in the North Atlantic Ocean, situated between Greenland and mainland Europe. It lies just south of the Arctic Circle, making it one of
  the northernmost countries in the world. Despite its name, Iceland has a relatively mild climate compared to other locations at similar latitudes, thanks to the North Atlantic
  Current. The capital and largest city is Reykjavík.
```

## How It Works

1. **Deployment**: When the gateway starts, it detects servers with `serverless` configuration
2. **kubectl apply**: Executes `kubectl apply -f <configPath>` to deploy the server
3. **Wait**: Waits for composition to reach Running phase
4. **Connect**: Creates a remote MCP client to connect via the specified `remote.url`
5. **Cleanup**: On shutdown, executes `kubectl delete -f <configPath>` to clean up

