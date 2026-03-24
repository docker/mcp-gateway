# docker mcp profile tools

<!---MARKER_GEN_START-->
Manage the tool allowlist for servers in a profile.
Tools are specified using dot notation: <serverName>.<toolName>

Use --enable to enable specific tools for a server (can be specified multiple times).
Use --disable to disable specific tools for a server (can be specified multiple times).
Use --enable-all to enable all tools for a server (can be specified multiple times).
Use --disable-all to disable all tools for a server (can be specified multiple times).

To view enabled tools, use: docker mcp profile show <profile-id>

### Options

| Name            | Type          | Default | Description                                                  |
|:----------------|:--------------|:--------|:-------------------------------------------------------------|
| `--disable`     | `stringArray` |         | Disable specific tools: <serverName>.<toolName> (repeatable) |
| `--disable-all` | `stringArray` |         | Disable all tools for a server: <serverName> (repeatable)    |
| `--enable`      | `stringArray` |         | Enable specific tools: <serverName>.<toolName> (repeatable)  |
| `--enable-all`  | `stringArray` |         | Enable all tools for a server: <serverName> (repeatable)     |


<!---MARKER_GEN_END-->

