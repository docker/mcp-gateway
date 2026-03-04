#!/usr/bin/env python3
import json
import re

# Load the skip reasons
with open('skip_reasons_to_analyze.json', 'r') as f:
    skip_reasons_data = json.load(f)

# Categorization logic
def categorize_skip_reason(skip_reason):
    """Categorize a skip reason based on keywords."""
    skip_lower = skip_reason.lower()

    # API key indicators
    api_key_indicators = [
        'api key', 'api_key', 'api token', 'access token', 'secret', 'credentials',
        'authentication', 'auth token', 'client_id', 'client_secret',
        'personal access token', 'bearer token', 'oauth', 'saas', 'cloud service',
        'paid api', 'paid/cloud', 'paid/saas', 'jwt token'
    ]

    # Hardware indicators
    hardware_indicators = [
        'physical', 'android device', 'adb', 'macos', 'apple', 'device connected',
        'hardware', 'scrcpy', 'usb', 'virtualization framework'
    ]

    # Licensed software indicators
    licensed_indicators = [
        'licensed', 'proprietary', 'commercial software', 'stata', 'paid product',
        'cisco modeling labs', 'cml is a paid'
    ]

    # Check for API keys (highest priority as it's most common)
    if any(indicator in skip_lower for indicator in api_key_indicators):
        return 'api_key_required'

    # Check for hardware
    if any(indicator in skip_lower for indicator in hardware_indicators):
        return 'hardware_required'

    # Check for licensed software
    if any(indicator in skip_lower for indicator in licensed_indicators):
        return 'licensed_software'

    return 'other'

# Categorize all skip reasons
categorized = []
category_counts = {
    'api_key_required': 0,
    'hardware_required': 0,
    'licensed_software': 0,
    'other': 0
}

for item in skip_reasons_data:
    category = categorize_skip_reason(item['skip_reason'])
    categorized.append({
        'name': item['name'],
        'category': category,
        'skip_reason': item['skip_reason']
    })
    category_counts[category] += 1

# Sort by category
categorized.sort(key=lambda x: (x['category'], x['name']))

# Print summary
print("="*70)
print("CATEGORIZATION SUMMARY")
print("="*70)
print(f"\nTotal servers analyzed: {len(categorized)}\n")

for category, count in sorted(category_counts.items(), key=lambda x: -x[1]):
    percentage = (count / len(categorized) * 100) if len(categorized) > 0 else 0
    print(f"{category:<25}: {count:3d} ({percentage:5.2f}%)")

# Show examples from each category
print("\n" + "="*70)
print("EXAMPLES BY CATEGORY")
print("="*70)

for category in ['api_key_required', 'hardware_required', 'licensed_software', 'other']:
    servers_in_category = [s for s in categorized if s['category'] == category]

    if not servers_in_category:
        continue

    print(f"\n{'='*70}")
    print(f"{category.upper().replace('_', ' ')} ({len(servers_in_category)} servers)")
    print('='*70)

    for i, server in enumerate(servers_in_category[:5], 1):
        print(f"\n{i}. {server['name']}")
        # Show first 150 chars of skip reason
        reason_short = server['skip_reason'][:150]
        if len(server['skip_reason']) > 150:
            reason_short += "..."
        print(f"   {reason_short}")

    if len(servers_in_category) > 5:
        print(f"\n   ... and {len(servers_in_category) - 5} more")

# Save categorized results
with open('categorized_skip_reasons.json', 'w') as f:
    json.dump(categorized, f, indent=2)

print("\n" + "="*70)
print("\nSaved detailed categorization to categorized_skip_reasons.json")

# Create summary for easy consumption
summary = {
    'total_servers': len(categorized),
    'categories': category_counts,
    'api_key_servers': [s['name'] for s in categorized if s['category'] == 'api_key_required']
}

with open('categorization_summary.json', 'w') as f:
    json.dump(summary, f, indent=2)

print("Saved summary to categorization_summary.json")
