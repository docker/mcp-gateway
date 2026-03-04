#!/usr/bin/env python3
import json
from collections import Counter

# Load the results.json file
with open('results.json', 'r') as f:
    data = json.load(f)

# Collect all unique fields
all_fields = set()
for server in data:
    all_fields.update(server.keys())

print("All fields found in the data:")
for field in sorted(all_fields):
    print(f"  - {field}")

print("\n" + "="*50)
print("Sample entry:")
print(json.dumps(data[0], indent=2))
