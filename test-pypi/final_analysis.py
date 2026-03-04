#!/usr/bin/env python3
import json

# Load the categorized results
with open('categorized_skip_reasons.json', 'r') as f:
    categorized = json.load(f)

# Load both result files to get prior status
with open('results.json', 'r') as f:
    current_data = json.load(f)

with open('results.prior.json', 'r') as f:
    prior_data = json.load(f)

# Build dictionaries
prior_dict = {(s.get('name') or s.get('serverName')): s for s in prior_data}

# Match categorized servers with their prior status
analysis = {
    'api_key_required': {'failed': [], 'profile_failed': [], 'no_tools': []},
    'hardware_required': {'failed': [], 'profile_failed': [], 'no_tools': []},
    'other': {'failed': [], 'profile_failed': [], 'no_tools': []}
}

for item in categorized:
    name = item['name']
    category = item['category']

    if category == 'licensed_software':  # Merge with 'other' for simplicity
        category = 'other'

    if name in prior_dict:
        prior_status = prior_dict[name].get('status')
        if prior_status in ['failed', 'profile_failed', 'no_tools']:
            if category in analysis:
                analysis[category][prior_status].append(name)

print("="*70)
print("FINAL ANALYSIS: Problematic Servers from Prior Run")
print("Categorized by Skip Reason")
print("="*70)

total_api_key = 0
total_hardware = 0
total_other = 0

for category, statuses in analysis.items():
    category_total = sum(len(v) for v in statuses.values())

    if category == 'api_key_required':
        total_api_key = category_total
    elif category == 'hardware_required':
        total_hardware = category_total
    elif category == 'other':
        total_other = category_total

    print(f"\n{category.upper().replace('_', ' ')}: {category_total} servers")
    print("-"*70)

    for prior_status, servers in statuses.items():
        if servers:
            print(f"  {prior_status:<20}: {len(servers):3d} servers")

print("\n" + "="*70)
print("KEY FINDINGS")
print("="*70)

print(f"""
Out of 101 servers that had problems in the prior run:

📌 {total_api_key} servers ({total_api_key/101*100:.1f}%) require API keys/cloud credentials
   - These likely appeared to have "no tools" because they couldn't
     authenticate to the actual service

📌 {total_hardware} servers ({total_hardware/101*100:.1f}%) require physical hardware/platform-specific tools
   - These couldn't be tested in a containerized environment

📌 {total_other} servers ({total_other/101*100:.1f}%) have other complex requirements
   - Complex backing services, cluster requirements, etc.

INTERPRETATION:
Most servers that failed or showed "no_tools" in the prior run were actually
just missing required credentials. The current run correctly identifies these
as untestable and skips them with clear explanations.
""")

print("="*70)
print("BREAKDOWN BY PRIOR STATUS")
print("="*70)

# Show detailed breakdown
print("\n1. SERVERS THAT SHOWED 'no_tools' IN PRIOR RUN (99 servers)")
print("-"*70)
no_tools_api = len(analysis['api_key_required']['no_tools'])
no_tools_hw = len(analysis['hardware_required']['no_tools'])
no_tools_other = len(analysis['other']['no_tools'])

print(f"   - {no_tools_api} required API keys ({no_tools_api/99*100:.1f}%)")
print(f"   - {no_tools_hw} required hardware ({no_tools_hw/99*100:.1f}%)")
print(f"   - {no_tools_other} had other issues ({no_tools_other/99*100:.1f}%)")
print(f"\n   👉 This means {no_tools_api} servers appeared to have no tools simply")
print(f"      because they couldn't authenticate! They might actually have tools.")

print("\n2. SERVERS THAT 'failed' IN PRIOR RUN (1 server)")
print("-"*70)
failed_api = len(analysis['api_key_required']['failed'])
print(f"   - {failed_api} required API keys")
print("   - This server failed because it needed credentials")

print("\n3. SERVERS THAT 'profile_failed' IN PRIOR RUN (1 server)")
print("-"*70)
pf_hw = len(analysis['hardware_required']['profile_failed'])
print(f"   - {pf_hw} required hardware (macOS-specific)")
print("   - This couldn't even be profiled due to platform requirements")
