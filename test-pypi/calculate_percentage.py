#!/usr/bin/env python3
import json

# Load the results.json file
with open('results.json', 'r') as f:
    data = json.load(f)

# Count servers with tools
total_servers = len(data)
servers_with_tools = 0
servers_with_tools_found = 0
servers_with_no_tools = 0

for server in data:
    # Check if server has tools (not empty tools list)
    if 'tools' in server and server['tools']:
        servers_with_tools += 1

    # Check for tools_found field
    if 'tools_found' in server:
        servers_with_tools_found += 1

    # Check for no_tools field
    if 'no_tools' in server:
        servers_with_no_tools += 1

# Calculate percentage
percentage = (servers_with_tools / total_servers * 100) if total_servers > 0 else 0

print(f"Total servers: {total_servers}")
print(f"Servers with tools: {servers_with_tools}")
print(f"Percentage: {percentage:.2f}%")
print()
print(f"Servers with 'tools_found' field: {servers_with_tools_found}")
print(f"Servers with 'no_tools' field: {servers_with_no_tools}")
