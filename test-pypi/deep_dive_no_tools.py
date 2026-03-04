#!/usr/bin/env python3
import json

# Load the analysis results
with open('remaining_no_tools_analysis.json', 'r') as f:
    remaining = json.load(f)

# Load both result files for full context
with open('results.json', 'r') as f:
    current_data = json.load(f)

with open('results.prior.json', 'r') as f:
    prior_data = json.load(f)

current_dict = {(s.get('name') or s.get('serverName')): s for s in current_data}

# Focus on servers that still show no_tools
still_no_tools = [s for s in remaining if s['current_status'] == 'no_tools']

print("="*70)
print("DEEP DIVE: 72 SERVERS THAT GENUINELY HAVE 'no_tools'")
print("="*70)
print("\nThese servers show 'no_tools' in BOTH runs with NO errors.")
print("Let's examine what information we have about them:\n")

# Get full details for these servers
detailed_servers = []
for server in still_no_tools:
    name = server['name']
    if name in current_dict:
        current = current_dict[name]
        detailed_servers.append({
            'name': name,
            'serverUrl': current.get('serverUrl', ''),
            'hasServices': current.get('hasServices'),
            'toolCount': current.get('toolCount', 0),
            'tools': current.get('tools', []),
            'error': current.get('error', ''),
            'skipReason': current.get('skipReason', '')
        })

# Check if they have services (prompts/resources)
has_services_count = sum(1 for s in detailed_servers if s['hasServices'])
no_services_count = sum(1 for s in detailed_servers if not s['hasServices'])

print("="*70)
print("SERVICE AVAILABILITY")
print("="*70)
print(f"\nServers with hasServices=true: {has_services_count}")
print(f"Servers with hasServices=false: {no_services_count}")
print("\nNote: hasServices indicates if the server has prompts or resources,")
print("      but no tools.")

# Analyze by package pattern
print("\n" + "="*70)
print("PACKAGE NAME PATTERNS")
print("="*70)

# Group by common package prefixes
patterns = {}
for server in detailed_servers:
    name = server['name']
    # Extract package prefix
    if name.startswith('io.github.'):
        parts = name.replace('io.github.', '').split('/')
        if len(parts) > 0:
            author = parts[0]
            if author not in patterns:
                patterns[author] = []
            patterns[author].append(name)
    else:
        # Other namespace
        parts = name.split('/')
        if len(parts) > 0:
            prefix = parts[0]
            if prefix not in patterns:
                patterns[prefix] = []
            patterns[prefix].append(name)

# Show authors with multiple packages
multi_package_authors = {k: v for k, v in patterns.items() if len(v) > 1}

if multi_package_authors:
    print("\nAuthors with multiple packages showing no_tools:")
    for author, packages in sorted(multi_package_authors.items(), key=lambda x: -len(x[1]))[:10]:
        print(f"\n  {author}: {len(packages)} packages")
        for pkg in packages[:5]:
            print(f"    - {pkg}")
        if len(packages) > 5:
            print(f"    ... and {len(packages) - 5} more")

# Sample some specific servers to examine
print("\n" + "="*70)
print("SAMPLE SERVERS WITH NO TOOLS")
print("="*70)

sample_servers = detailed_servers[:15]
for i, server in enumerate(sample_servers, 1):
    print(f"\n{i}. {server['name']}")
    print(f"   URL: {server['serverUrl']}")
    print(f"   Has services (prompts/resources): {server['hasServices']}")
    print(f"   Tool count: {server['toolCount']}")
    if server['error']:
        print(f"   Error: {server['error'][:100]}")

# Summary insight
print("\n" + "="*70)
print("POSSIBLE EXPLANATIONS")
print("="*70)
print("""
These 72 servers consistently show 'no_tools' with no errors. Possible reasons:

1. **Genuinely No Tools**: The server only provides prompts or resources
   - {has_services} servers have hasServices=true (they provide prompts/resources)
   - These may be intentionally tool-free servers

2. **Registration Issue**: The server's package.json or metadata doesn't
   correctly declare tools, so the profiler can't find them

3. **Runtime Requirements**: The server needs specific runtime conditions
   to initialize tools (environment variables, config files, etc.) but
   doesn't fail - it just doesn't expose tools

4. **Conditional Tools**: The server only exposes tools under certain
   conditions (e.g., if certain optional dependencies are present)

5. **Version-Specific**: These specific versions may have bugs or
   be empty placeholder releases

To determine the true reason, you would need to:
- Manually inspect the source code repositories
- Check the package.json tool declarations
- Review the server implementation to see if tools are conditionally registered
- Check if these servers are meant to be tool-free (prompts/resources only)
""".format(has_services=has_services_count))

# Export the list for further investigation
with open('genuine_no_tools_servers.json', 'w') as f:
    json.dump(detailed_servers, f, indent=2)

print("\nSaved detailed list to genuine_no_tools_servers.json")
