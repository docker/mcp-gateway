# Interceptors (beta)

Interceptors are extension points. They can mediate tool calls for any active MCP running in the gateway. This feature should be considered beta, and subject to change as we learn from users what they need to manage their MCP workloads.

## **after** Tool Calls

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

```json
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

The stdout must either be empty or it must be able to be parsed to a valid `ToolCall` response. These interceptors are powerful. They can easily blow up your entire MCP workload.

* we do not currently care about exit codes
* stdout must contain valid `application/json` that can be parsed to a ToolCall response
* if the script writes nothing to stdout, the existing response will be passed on.

## **before** Tool Calls

Analogously, run with a before interceptor.

```bash
docker mcp gateway run --interceptor "before:exec:$HOME/script.sh"
```

For _before_ interceptors, only the request value will written to stdin. Before interceptors are potentially more impactful than _after_ interceptors. If they return anything on stdout then two things will happen.

1. the handler chain will prematurely end (no tool call will be made)
2. the response payload from the interceptor will _become_ the tool call response. This means that _before_ interceptors can reject tool calls and add their own tool call responses.

