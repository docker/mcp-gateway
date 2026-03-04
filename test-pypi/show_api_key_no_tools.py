#!/usr/bin/env python3
import json

# Load the categorized results
with open('categorized_skip_reasons.json', 'r') as f:
    categorized = json.load(f)

# Load prior results
with open('results.prior.json', 'r') as f:
    prior_data = json.load(f)

# Build dictionary
prior_dict = {(s.get('name') or s.get('serverName')): s for s in prior_data}

# Find servers that were "no_tools" in prior but need API keys
api_key_no_tools = []

for item in categorized:
    if item['category'] == 'api_key_required':
        name = item['name']
        if name in prior_dict and prior_dict[name].get('status') == 'no_tools':
            api_key_no_tools.append({
                'name': name,
                'skip_reason': item['skip_reason'],
                'prior_error': prior_dict[name].get('error', 'No error message')
            })

print("="*70)
print("SERVERS THAT SHOWED 'no_tools' BUT REQUIRE API KEYS")
print("="*70)
print(f"\nTotal: {len(api_key_no_tools)} servers")
print("\nThese servers likely DO have tools, but couldn't show them")
print("because they couldn't authenticate to their service.\n")
print("="*70)

# Show a sample of interesting ones
for i, server in enumerate(api_key_no_tools[:15], 1):
    print(f"\n{i}. {server['name']}")

    # Extract key requirements from skip reason
    skip_reason = server['skip_reason']

    # Try to find what API keys are needed
    import re
    env_vars = re.findall(r'[A-Z_]{3,}(?:_KEY|_TOKEN|_SECRET|_ID|_PASSWORD|_API|_URL)', skip_reason)

    if env_vars:
        unique_vars = list(set(env_vars))
        print(f"   Required env vars: {', '.join(unique_vars[:5])}")
        if len(unique_vars) > 5:
            print(f"   ... and {len(unique_vars) - 5} more")

    # Show first line of skip reason
    first_line = skip_reason.split('.')[0]
    if len(first_line) > 100:
        first_line = first_line[:100] + "..."
    print(f"   Reason: {first_line}")

if len(api_key_no_tools) > 15:
    print(f"\n... and {len(api_key_no_tools) - 15} more servers")

print("\n" + "="*70)
print("\nCONCLUSION:")
print("These 79 servers appeared to have 'no_tools' in the prior run,")
print("but this was likely a FALSE NEGATIVE caused by missing credentials.")
print("The servers may actually have tools, but couldn't discover them")
print("without valid API keys to connect to their services.")
print("="*70)
