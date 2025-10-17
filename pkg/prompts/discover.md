---
description: Discover relevant MCP servers for your current project - 5
argument-hint: "[project-path]"
allowed-tools: ["Task", "Read", "Glob", Bash(docker mcp:*)]
---

# Discover Relevant MCP Servers

Analyze your project and get personalized MCP server recommendations based on your actual dependencies and tech stack.

---

## Prerequisites Check

Check if mcp-find tool is available (indicates dynamic-tools feature is enabled).

If NOT available:
```
⚠️ This command requires dynamic-tools enabled.

Enable it: docker mcp feature enable dynamic-tools
Then restart Claude Code.
```

---

## Project Detection

Use Glob to check for project files:
- package.json, requirements.txt, go.mod, Cargo.toml, etc.
- README.md
- .git directory

If no project detected, ask user for project path or continue with generic recommendations.

If project detected:
```
✓ Project detected
Analyzing to find relevant MCP servers...
```

---

## Launch Agents in Parallel

Launch 3 specialized agents simultaneously for faster results:

```
Use Task tool to launch ALL 3 agents in parallel:

1. mcp-discover-packages
   → Analyzes package.json dependencies
   → Calls mcp-find for each package

2. mcp-discover-readme
   → Analyzes README.md service mentions
   → Calls mcp-find for each mention

3. mcp-discover-defaults
   → Determines always-suggest servers
   → github-official (if .git), playwright (if web app), context7 (always)

All run simultaneously → faster results (~10-15 seconds)
```

Wait for all 3 agents to complete...

---

## Merge Agent Results

Receive data from all 3 agents:

**packages_agent.matched_servers**: Servers matching package.json dependencies
**readme_agent.matched_servers**: Servers from README mentions
**defaults_agent.default_servers**: Always-suggest servers

**Combine**:
```
all_servers = []

Add all from packages_agent.matched_servers → Recommended
Add all from readme_agent.matched_servers → Recommended
Add all from defaults_agent.default_servers:
  - github-official → Recommended (if returned)
  - playwright, context7 → Suggested

Deduplicate by server name
```

**Result**: Combined list of recommended + suggested servers

---

## Format and Present

Transform agent data into user-friendly output:

```
┌─────────────────────────────────────────────────────┐
│ MCP Server Discovery Results                       │
└─────────────────────────────────────────────────────┘

Files Analyzed:
{for each file in FILES_READ}
- ✓ {file}

Searches Executed:
{for each search in SEARCHES_EXECUTED}
- {query} → {matches} matches {if matches > 0: list server names}

Project Summary:
{PROJECT_SUMMARY}

---

⭐️ Recommended

{for each server in RECOMMENDED_SERVERS}
• {name}
  - Found in: {found_in}
  - Capabilities: {description}
  - Setup: {if oauth: "OAuth - Run: docker mcp oauth authorize {name}"}
          {else if secrets: "Requires: {join(secrets, ', ')}"}
          {else: "No setup needed"}

💡 Suggested

{for each server in SUGGESTED_SERVERS}
• {name}
  - Why: {reason}
  - Capabilities: {description}
  - Setup: {same logic as above}

---

Summary:
- Files read: {count FILES_READ}
- Searches performed: {count SEARCHES_EXECUTED}
- Servers found: {count RECOMMENDED + SUGGESTED}
```

---

## Interactive Selection

Ask user:
```
What would you like to do?

1. Enable all recommended servers
2. Enable specific servers
3. Exit

Your choice:
```

Based on selection:
- Option 1: Enable all from RECOMMENDED_SERVERS
- Option 2: Show numbered list, user selects, enable selected
- Option 3: Exit

---

## Enable Servers

If user approved, enable each selected server using Bash (command has Bash access, agents don't):
We must use: `docker mcp server enable` we should NOT use mcp-add due to tool issues

```bash
For each selected server:
  docker mcp server enable <server-name>

Example:
  docker mcp server enable neon
  docker mcp server enable redis
  docker mcp server enable playwright
  docker mcp server enable github-official
```

Show progress as each completes:
```
Enabling neon... ✓
Enabling redis... ✓
Enabling playwright... ✓
Enabling github-official... ✓
```

---

## Summary

Show final summary:
- How many servers enabled
- Restart notice (IMPORTANT!)
- Which secrets need configuration
- Next steps

```
✓ Enabled X servers (permanently)

⚠️ IMPORTANT: Restart Claude Code to activate these servers

Steps:
1. Exit Claude Code (Ctrl+C or /exit)
2. Restart: claude
3. Your new tools will be available!

After restart:
- Verify: /docker-mcp-toolkit:gateway-status
- Configure secrets if needed
```
