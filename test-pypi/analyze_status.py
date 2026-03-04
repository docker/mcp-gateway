#!/usr/bin/env python3
import json
from collections import Counter

# Load the results.json file
with open('results.json', 'r') as f:
    data = json.load(f)

# Count status values
status_counter = Counter()
for server in data:
    if 'status' in server:
        status_counter[server['status']] += 1

total_servers = len(data)

print(f"Total servers: {total_servers}\n")
print("Status breakdown:")
print("="*50)
for status, count in status_counter.most_common():
    percentage = (count / total_servers * 100)
    print(f"{status:20s}: {count:3d} ({percentage:5.2f}%)")

print("\n" + "="*50)
# Specific counts
tools_found = status_counter.get('tools_found', 0)
no_tools = status_counter.get('no_tools', 0)

print(f"\nServers with 'tools_found' status: {tools_found}")
print(f"Servers with 'no_tools' status: {no_tools}")
print(f"Combined: {tools_found + no_tools} ({(tools_found + no_tools) / total_servers * 100:.2f}%)")
