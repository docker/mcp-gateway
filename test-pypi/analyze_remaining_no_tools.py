#!/usr/bin/env python3
import json

# Load both files
with open('results.json', 'r') as f:
    current_data = json.load(f)

with open('results.prior.json', 'r') as f:
    prior_data = json.load(f)

# Load categorized skip reasons to know which ones we already analyzed
with open('categorized_skip_reasons.json', 'r') as f:
    categorized = json.load(f)

analyzed_names = {item['name'] for item in categorized}

# Build dictionaries
current_dict = {(s.get('name') or s.get('serverName')): s for s in current_data}
prior_dict = {(s.get('name') or s.get('serverName')): s for s in prior_data}

# Find servers that had "no_tools" in prior run
prior_no_tools = [name for name, server in prior_dict.items()
                  if server.get('status') == 'no_tools']

print("="*70)
print("SERVERS WITH 'no_tools' IN PRIOR RUN")
print("="*70)
print(f"\nTotal servers with no_tools in prior run: {len(prior_no_tools)}")
print(f"Servers we already analyzed (now skipped): {len(analyzed_names & set(prior_no_tools))}")
print(f"Remaining servers to investigate: {len(set(prior_no_tools) - analyzed_names)}")

# Investigate the remaining servers
remaining_no_tools = []
for name in prior_no_tools:
    if name not in analyzed_names:
        prior_server = prior_dict[name]

        # Check current status
        if name in current_dict:
            current_server = current_dict[name]
            remaining_no_tools.append({
                'name': name,
                'prior_status': 'no_tools',
                'current_status': current_server.get('status'),
                'prior_error': prior_server.get('error', ''),
                'current_error': current_server.get('error', ''),
                'current_tool_count': current_server.get('toolCount', 0),
                'skip_reason': current_server.get('skipReason', '')
            })
        else:
            # Server not in current run
            remaining_no_tools.append({
                'name': name,
                'prior_status': 'no_tools',
                'current_status': 'NOT_IN_CURRENT_RUN',
                'prior_error': prior_server.get('error', ''),
                'current_error': '',
                'current_tool_count': 0,
                'skip_reason': ''
            })

# Group by current status
by_current_status = {}
for server in remaining_no_tools:
    status = server['current_status']
    if status not in by_current_status:
        by_current_status[status] = []
    by_current_status[status].append(server)

print("\n" + "="*70)
print("BREAKDOWN OF REMAINING 'no_tools' SERVERS BY CURRENT STATUS")
print("="*70)

for status, servers in sorted(by_current_status.items(), key=lambda x: -len(x[1])):
    print(f"\n{status}: {len(servers)} servers")
    print("-"*70)

    # Show examples
    for i, server in enumerate(servers[:5], 1):
        print(f"\n{i}. {server['name']}")
        if server['prior_error']:
            error_short = server['prior_error'][:150]
            if len(server['prior_error']) > 150:
                error_short += "..."
            print(f"   Prior error: {error_short}")
        if server['current_error']:
            error_short = server['current_error'][:150]
            if len(server['current_error']) > 150:
                error_short += "..."
            print(f"   Current error: {error_short}")
        if server['current_tool_count'] > 0:
            print(f"   Current tool count: {server['current_tool_count']}")

    if len(servers) > 5:
        print(f"\n   ... and {len(servers) - 5} more servers")

# Focus on servers still showing no_tools
print("\n" + "="*70)
print("DETAILED ANALYSIS: SERVERS STILL SHOWING 'no_tools' IN CURRENT RUN")
print("="*70)

still_no_tools = by_current_status.get('no_tools', [])
print(f"\nTotal: {len(still_no_tools)} servers")
print("\nThese servers consistently show no tools across both runs.")
print("Let's examine their errors to understand why:\n")

# Analyze error patterns
error_patterns = {}
for server in still_no_tools:
    error = server['prior_error'] or server['current_error'] or 'No error message'

    # Categorize error
    error_lower = error.lower()
    if 'permission' in error_lower or 'denied' in error_lower:
        category = 'Permission/Access Denied'
    elif 'timeout' in error_lower or 'timed out' in error_lower:
        category = 'Timeout'
    elif 'connection' in error_lower or 'connect' in error_lower:
        category = 'Connection Error'
    elif 'not found' in error_lower or '404' in error_lower:
        category = 'Not Found'
    elif 'no error' in error_lower.lower():
        category = 'No Error (Genuinely No Tools?)'
    elif error == '':
        category = 'No Error (Genuinely No Tools?)'
    else:
        category = 'Other Error'

    if category not in error_patterns:
        error_patterns[category] = []
    error_patterns[category].append(server)

print("Error pattern breakdown:")
for category, servers in sorted(error_patterns.items(), key=lambda x: -len(x[1])):
    print(f"\n{category}: {len(servers)} servers")
    for i, server in enumerate(servers[:3], 1):
        print(f"  {i}. {server['name']}")
        if server['prior_error'] or server['current_error']:
            error = server['prior_error'] or server['current_error']
            error_short = error[:100]
            if len(error) > 100:
                error_short += "..."
            print(f"     Error: {error_short}")

print("\n" + "="*70)
print("SUMMARY")
print("="*70)
print(f"""
Out of {len(prior_no_tools)} servers with 'no_tools' in prior run:

1. {len(analyzed_names & set(prior_no_tools))} servers are now SKIPPED with clear reasons
   (79 needed API keys, 10 needed hardware, 10 had other issues)

2. {len(still_no_tools)} servers STILL show 'no_tools' in current run
   - These appear to genuinely have no tools or have persistent issues

3. {len(by_current_status.get('tools_found', []))} servers now show 'tools_found'
   - Improvements in profiling logic!

4. {len(by_current_status.get('NOT_IN_CURRENT_RUN', []))} servers not in current run

5. Other status changes: {sum(len(v) for k, v in by_current_status.items()
                              if k not in ['no_tools', 'tools_found', 'NOT_IN_CURRENT_RUN'])} servers
""")

# Save detailed data
with open('remaining_no_tools_analysis.json', 'w') as f:
    json.dump(remaining_no_tools, f, indent=2)

print("\nSaved detailed analysis to remaining_no_tools_analysis.json")
