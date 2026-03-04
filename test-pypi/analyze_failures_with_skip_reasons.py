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

# Find servers that had issues in prior run
problem_servers = []

for name, prior_server in prior_dict.items():
    prior_status = prior_server.get('status')

    # Check if it was a problem status in prior run
    if prior_status in ['failed', 'profile_failed', 'no_tools']:
        # Check if it exists in current and has a skipReason
        if name in current_dict:
            current_server = current_dict[name]
            skip_reason = current_server.get('skipReason')

            if skip_reason:
                problem_servers.append({
                    'name': name,
                    'prior_status': prior_status,
                    'current_status': current_server.get('status'),
                    'skip_reason': skip_reason,
                    'prior_tool_count': prior_server.get('toolCount', 0),
                    'prior_error': prior_server.get('error', '')
                })

print("="*70)
print("SERVERS WITH PROBLEMS IN PRIOR RUN THAT NOW HAVE SKIP REASONS")
print("="*70)
print(f"\nTotal: {len(problem_servers)} servers\n")

# Group by prior status
by_prior_status = {}
for server in problem_servers:
    status = server['prior_status']
    if status not in by_prior_status:
        by_prior_status[status] = []
    by_prior_status[status].append(server)

for status, servers in sorted(by_prior_status.items()):
    print(f"\n{'='*70}")
    print(f"Prior status: {status} ({len(servers)} servers)")
    print('='*70)

    for i, server in enumerate(servers[:5], 1):  # Show first 5 of each category
        print(f"\n{i}. {server['name']}")
        print(f"   Prior status: {server['prior_status']}")
        print(f"   Current status: {server['current_status']}")
        print(f"   Skip reason: {server['skip_reason'][:200]}...")
        if server['prior_error']:
            print(f"   Prior error: {server['prior_error'][:200]}...")

    if len(servers) > 5:
        print(f"\n   ... and {len(servers) - 5} more servers with prior status '{status}'")

# Export all skip reasons for analysis
skip_reasons_for_analysis = [{'name': s['name'], 'skip_reason': s['skip_reason']} for s in problem_servers]

with open('skip_reasons_to_analyze.json', 'w') as f:
    json.dump(skip_reasons_for_analysis, f, indent=2)

print("\n" + "="*70)
print(f"\nExported {len(skip_reasons_for_analysis)} skip reasons to skip_reasons_to_analyze.json")
print("This file can be analyzed with Claude to categorize API key requirements")
