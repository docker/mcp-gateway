#!/usr/bin/env python3
import json
from collections import Counter

# Load both files
with open('results.json', 'r') as f:
    current_data = json.load(f)

with open('results.prior.json', 'r') as f:
    prior_data = json.load(f)

print("="*70)
print("COMPARISON: results.json vs results.prior.json")
print("="*70)

# Basic counts
print(f"\nTotal servers:")
print(f"  Current (results.json):       {len(current_data)}")
print(f"  Prior (results.prior.json):   {len(prior_data)}")
print(f"  Difference:                   {len(current_data) - len(prior_data)}")

# Status breakdown for current
current_status = Counter()
for server in current_data:
    if 'status' in server:
        current_status[server['status']] += 1

# Status breakdown for prior
prior_status = Counter()
for server in prior_data:
    if 'status' in server:
        prior_status[server['status']] += 1

print("\n" + "="*70)
print("STATUS BREAKDOWN COMPARISON")
print("="*70)
print(f"{'Status':<20} {'Current':<15} {'Prior':<15} {'Difference':<15}")
print("-"*70)

all_statuses = sorted(set(current_status.keys()) | set(prior_status.keys()))
for status in all_statuses:
    curr_count = current_status.get(status, 0)
    prior_count = prior_status.get(status, 0)
    diff = curr_count - prior_count
    curr_pct = (curr_count / len(current_data) * 100) if len(current_data) > 0 else 0
    prior_pct = (prior_count / len(prior_data) * 100) if len(prior_data) > 0 else 0

    print(f"{status:<20} {curr_count:3d} ({curr_pct:5.2f}%) {prior_count:3d} ({prior_pct:5.2f}%) {diff:+4d}")

# Tools analysis
current_tools = sum(1 for s in current_data if s.get('tools'))
prior_tools = sum(1 for s in prior_data if s.get('tools'))

print("\n" + "="*70)
print("TOOLS ANALYSIS")
print("="*70)
print(f"Servers with tools:")
print(f"  Current: {current_tools} ({current_tools/len(current_data)*100:.2f}%)")
print(f"  Prior:   {prior_tools} ({prior_tools/len(prior_data)*100:.2f}%)")
print(f"  Difference: {current_tools - prior_tools:+d}")

# Total tool count
current_tool_count = sum(s.get('toolCount', 0) for s in current_data)
prior_tool_count = sum(s.get('toolCount', 0) for s in prior_data)

print(f"\nTotal tool count:")
print(f"  Current: {current_tool_count}")
print(f"  Prior:   {prior_tool_count}")
print(f"  Difference: {current_tool_count - prior_tool_count:+d}")

# Check for servers in one but not the other
current_names = {s.get('name') or s.get('serverName') for s in current_data}
prior_names = {s.get('name') or s.get('serverName') for s in prior_data}

only_in_current = current_names - prior_names
only_in_prior = prior_names - current_names

print("\n" + "="*70)
print("SERVER DIFFERENCES")
print("="*70)
print(f"Servers only in current: {len(only_in_current)}")
if only_in_current and len(only_in_current) <= 10:
    for name in sorted(only_in_current):
        print(f"  + {name}")

print(f"\nServers only in prior: {len(only_in_prior)}")
if only_in_prior and len(only_in_prior) <= 10:
    for name in sorted(only_in_prior):
        print(f"  - {name}")

# Check for status changes in common servers
print("\n" + "="*70)
print("STATUS CHANGES FOR COMMON SERVERS")
print("="*70)

# Build dictionaries for quick lookup
current_dict = {(s.get('name') or s.get('serverName')): s for s in current_data}
prior_dict = {(s.get('name') or s.get('serverName')): s for s in prior_data}

status_changes = []
for name in current_names & prior_names:
    curr_status = current_dict[name].get('status')
    prior_status = prior_dict[name].get('status')
    if curr_status != prior_status:
        status_changes.append({
            'name': name,
            'prior': prior_status,
            'current': curr_status
        })

print(f"Total servers with status changes: {len(status_changes)}")

if status_changes:
    # Group by change type
    change_counter = Counter()
    for change in status_changes:
        change_type = f"{change['prior']} → {change['current']}"
        change_counter[change_type] += 1

    print("\nStatus change breakdown:")
    for change_type, count in change_counter.most_common():
        print(f"  {change_type:<40}: {count:3d}")

    # Show first 10 examples
    if len(status_changes) <= 15:
        print("\nExamples of status changes:")
        for change in status_changes[:15]:
            print(f"  {change['name'][:50]:<50} {change['prior']:<15} → {change['current']:<15}")
