---
description: Discover relevant MCP servers for your current project - 5
argument-hint: "[project-path]"
---

# Discover Relevant MCP Servers

Analyze your project and get personalized MCP server recommendations based on your actual dependencies and tech stack.

---

## Prerequisites Check

Check if the mcp-find and mcp-add tools are available (indicates dynamic-tools feature is enabled).

If NOT available:
```
⚠️ This command requires dynamic-tools enabled.

Enable it: docker mcp feature enable dynamic-tools
Then restart.
```

---

## Project Detection

Search for project files:
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

## Analyze Project

Analyze the project to determine what MCPs might be relevant. 

```
1. mcp-discover-packages
   → Analyzes package.json dependencies
   → Calls mcp-find for each package

2. mcp-discover-readme
   → Analyzes README.md service mentions
   → Calls mcp-find for each mention

3. mcp-discover-defaults
   → Determines always-suggest servers
   → github-official (if .git), playwright (if web app), context7 (always)

```

---

## Merge Agent Results

Look at the data from having analyzed the project.

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

If user approved, add each server to the session and work with the user to configure them.

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
- Let the user know if the agent needs to be restarted.
- Which secrets need configuration
- Next steps

```
✓ Enabled X servers
```
