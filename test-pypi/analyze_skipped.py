#!/usr/bin/env python3
import json

# Load both files
with open('results.json', 'r') as f:
    current_data = json.load(f)

with open('results.prior.json', 'r') as f:
    prior_data = json.load(f)

print("="*70)
print("DETAILED ANALYSIS OF SKIPPED SERVERS")
print("="*70)

# Get skipped servers from current
skipped_servers = [s for s in current_data if s.get('status') == 'skipped']

print(f"\nTotal skipped servers: {len(skipped_servers)}")

# Check skipReason field
skip_reasons = {}
for server in skipped_servers:
    reason = server.get('skipReason', 'no reason given')
    if reason not in skip_reasons:
        skip_reasons[reason] = []
    skip_reasons[reason].append(server.get('name') or server.get('serverName'))

print(f"\nSkip reasons breakdown:")
for reason, servers in sorted(skip_reasons.items(), key=lambda x: len(x[1]), reverse=True):
    print(f"\n  {reason}: {len(servers)} servers")
    if len(servers) <= 5:
        for name in servers:
            print(f"    - {name}")

# Build prior dictionary to see what these skipped servers were before
prior_dict = {(s.get('name') or s.get('serverName')): s for s in prior_data}

print("\n" + "="*70)
print("WHAT WERE THE SKIPPED SERVERS IN THE PRIOR RUN?")
print("="*70)

skipped_names = [s.get('name') or s.get('serverName') for s in skipped_servers]
prior_status_of_skipped = {}

for name in skipped_names:
    if name in prior_dict:
        prior_status = prior_dict[name].get('status')
        prior_tool_count = prior_dict[name].get('toolCount', 0)
        if prior_status not in prior_status_of_skipped:
            prior_status_of_skipped[prior_status] = {'count': 0, 'tool_count': 0}
        prior_status_of_skipped[prior_status]['count'] += 1
        prior_status_of_skipped[prior_status]['tool_count'] += prior_tool_count

print("\nPrior status of currently skipped servers:")
for status, data in sorted(prior_status_of_skipped.items(), key=lambda x: x[1]['count'], reverse=True):
    print(f"  {status:<20}: {data['count']:3d} servers (had {data['tool_count']} total tools)")

# Show some examples of skipped servers that had tools before
print("\n" + "="*70)
print("EXAMPLES: Skipped servers that HAD TOOLS in prior run")
print("="*70)

skipped_with_prior_tools = []
for name in skipped_names:
    if name in prior_dict:
        prior = prior_dict[name]
        if prior.get('status') == 'tools_found' and prior.get('toolCount', 0) > 0:
            skipped_with_prior_tools.append({
                'name': name,
                'tool_count': prior.get('toolCount', 0),
                'tools': prior.get('tools', [])
            })

# Sort by tool count
skipped_with_prior_tools.sort(key=lambda x: x['tool_count'], reverse=True)

print(f"\nFound {len(skipped_with_prior_tools)} servers with tools that are now skipped")
print("\nTop 10 examples:")
for i, server in enumerate(skipped_with_prior_tools[:10], 1):
    print(f"\n{i}. {server['name']}")
    print(f"   Tool count: {server['tool_count']}")
    if server['tools']:
        # Handle tools being either list of dicts or list of strings
        tool_names = []
        for t in server['tools'][:3]:
            if isinstance(t, dict):
                tool_names.append(t.get('name', 'unnamed'))
            else:
                tool_names.append(str(t))
        print(f"   Sample tools: {', '.join(tool_names)}")
        if len(server['tools']) > 3:
            print(f"   ... and {len(server['tools']) - 3} more")
