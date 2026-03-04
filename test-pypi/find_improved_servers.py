#!/usr/bin/env python3
import json

# Load both files
with open('results.json', 'r') as f:
    current_data = json.load(f)

with open('results.prior.json', 'r') as f:
    prior_data = json.load(f)

# Build dictionaries for quick lookup
current_dict = {(s.get('name') or s.get('serverName')): s for s in current_data}
prior_dict = {(s.get('name') or s.get('serverName')): s for s in prior_data}

# Find servers that went from no_tools to tools_found
improved_servers = []

for name in set(current_dict.keys()) & set(prior_dict.keys()):
    prior_server = prior_dict[name]
    current_server = current_dict[name]

    prior_status = prior_server.get('status')
    current_status = current_server.get('status')

    if prior_status == 'no_tools' and current_status == 'tools_found':
        improved_servers.append({
            'name': name,
            'serverUrl': current_server.get('serverUrl', 'N/A'),
            'prior_tool_count': prior_server.get('toolCount', 0),
            'current_tool_count': current_server.get('toolCount', 0),
            'tools': current_server.get('tools', [])
        })

print("="*70)
print("SERVERS THAT IMPROVED: no_tools → tools_found")
print("="*70)
print(f"\nFound {len(improved_servers)} servers\n")

for i, server in enumerate(improved_servers, 1):
    print(f"{i}. {server['name']}")
    print(f"   URL: {server['serverUrl']}")
    print(f"   Prior tool count: {server['prior_tool_count']}")
    print(f"   Current tool count: {server['current_tool_count']}")
    print(f"   Improvement: +{server['current_tool_count'] - server['prior_tool_count']} tools")

    if server['tools']:
        print(f"\n   Tools found:")
        for tool in server['tools']:
            if isinstance(tool, dict):
                tool_name = tool.get('name', 'unnamed')
                tool_desc = tool.get('description', '')
                print(f"     - {tool_name}")
                if tool_desc:
                    # Truncate long descriptions
                    desc_short = tool_desc[:100] + "..." if len(tool_desc) > 100 else tool_desc
                    print(f"       {desc_short}")
            else:
                print(f"     - {tool}")
    print()
