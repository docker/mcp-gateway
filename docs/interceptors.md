# Interceptors (beta)

Interceptors are extension points. They can mediate tool calls for any active MCP running in the gateway. This feature should be considered beta, and subject to change as we learn from users what they need to manage their MCP workloads.

## **After** Tool Calls

A simple method to experiment with interceptors is to register a script to run after a tool call has completed but before the response is sent back to the client. Start the gateway with the `--interceptor` argument and provide a local script.

```bash
docker mcp gateway run --interceptor "after:exec:$HOME/script.sh"
```

This would not make a good production configuration as it relies on an external script but this is a useful way to try out an interceptor on real MCP traffic. The interceptor script will receive a json payload on stdin, and _must_ return the possibly edited response payload on stdout. Here's a simple that just echos the response.

```bash
#!/bin/bash

# Read JSON from stdin into a variable
json_input=$(cat)

# Extract the response property and serialize it back to JSON
echo "$json_input" | jq -r '.response | tostring'
```

The incoming request will have both `request` and `response` properties.

```
{
  "request": {
    "Session": {},
    "Params": {
      "_meta": {
        "progressToken": 1
      },
      "name": "get_me",
      "arguments": {}
    },
    "Extra": null
  },
  "response": {
    "content": [
      {
        "type": "text",
        "text": "..."
      }
    ]
  }
}

```

The interceptor must return a valid `ToolCall` response. Interceptors can easily blow your entire MCP workload. They can edit the response and are thus very powerful.

## **before** Tool Calls

