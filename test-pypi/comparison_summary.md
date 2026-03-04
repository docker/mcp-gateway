# Comparison Summary: results.json vs results.prior.json

## Key Findings

### Overall Statistics

| Metric | Current (results.json) | Prior (results.prior.json) | Difference |
|--------|------------------------|----------------------------|------------|
| Total servers | 279 | 311 | -32 |
| Servers with tools | 55 (19.71%) | 106 (34.08%) | -51 (-14.37%) |
| Total tool count | 821 | 1,539 | -718 (-46.7%) |

### Critical Observation

**The current run skipped 140 servers (50.18%) while the prior run skipped 0 servers.**

This is the primary cause of the differences:
- 39 servers that had tools in the prior run are now skipped (569 tools lost)
- 99 servers that had no tools in the prior run are now skipped
- 2 servers that were in other states are now skipped

## Status Breakdown Comparison

| Status | Current Count | Current % | Prior Count | Prior % | Change |
|--------|--------------|-----------|-------------|---------|--------|
| skipped | 140 | 50.18% | 0 | 0.00% | +140 |
| no_tools | 72 | 25.81% | 196 | 63.02% | -124 |
| tools_found | 55 | 19.71% | 106 | 34.08% | -51 |
| profile_failed | 9 | 3.23% | 4 | 1.29% | +5 |
| failed | 3 | 1.08% | 5 | 1.61% | -2 |

## Status Changes for Common Servers

148 servers changed status between runs:

| Status Change | Count |
|--------------|-------|
| no_tools → skipped | 99 |
| tools_found → skipped | 39 |
| no_tools → profile_failed | 6 |
| no_tools → tools_found | 2 |
| failed → skipped | 1 |
| profile_failed → skipped | 1 |

## Why Servers Were Skipped

All 140 skipped servers were skipped because they require credentials/resources that cannot be provided in a test environment:

### Breakdown by Skip Reason Category:

1. **Paid/Cloud SaaS API Keys** (~60% of skipped)
   - OpenAI, Anthropic, GitHub, Spotify, PagerDuty, etc.
   - Example: "Requires OPENAI_API_KEY (paid SaaS API key)"

2. **Cloud/Enterprise Service Credentials** (~25%)
   - Canvas LMS, ArcGIS, Microsoft Defender, Neo4j Aura, etc.
   - Example: "Requires Canvas LMS API credentials (CANVAS_API_TOKEN)"

3. **Physical Hardware Requirements** (~5%)
   - Android devices, Apple container tools, etc.
   - Example: "Requires a physical Android device connected via ADB"

4. **Proprietary/Licensed Software** (~5%)
   - Stata, Cisco Modeling Labs, etc.
   - Example: "Requires a licensed installation of Stata"

5. **Other Complex Dependencies** (~5%)
   - Proxy servers, security services, etc.

## Impact on Tools Found

### Top 10 Servers with Tools That Were Skipped:

1. **reclaim-mcp-server** (40 tools) - Requires Reclaim.ai API key
2. **pagerduty-mcp** (37 tools) - Requires PagerDuty API key
3. **open-notebook** (33 tools, 2 versions) - Requires OpenAI/Anthropic keys
4. **spotify-bulk-actions-mcp** (32 tools) - Requires Spotify API credentials
5. **u2-mcp** (31 tools) - Requires Rocket U2 database credentials
6. **galaxy-mcp** (31 tools) - Requires Galaxy platform credentials
7. **sauce-api-mcp** (30 tools) - Requires Sauce Labs credentials
8. **odsbox-jaquel-mcp** (29 tools) - Requires ODS Box credentials
9. **google-sheets-mcp** (24 tools) - Requires Google Sheets API credentials

**Total: 569 tools lost from these 39 skipped servers**

## Missing Servers

32 servers that were in the prior run are completely missing from the current run (not even marked as skipped).

## Interpretation

The current results.json represents a **filtered/conservative run** that only attempts to profile servers that can be tested with local backing services (databases, file systems, etc.) and excludes servers requiring:
- External API keys
- Cloud service credentials
- Physical hardware
- Proprietary software licenses

This filtering approach:
- ✅ Provides more accurate results for testable servers
- ✅ Avoids false negatives from credential-related failures
- ✅ Clearly documents why servers can't be tested
- ❌ Reduces the total number of servers tested from 311 to 279 (-10.3%)
- ❌ Reduces tools found from 106 to 55 (-48.1%)
- ❌ Reduces total tool count from 1,539 to 821 (-46.7%)

## Positive Changes

Despite the filtering, there were **2 servers that changed from no_tools to tools_found**, indicating:
- Improved profiling logic
- Better backing service setup
- Bug fixes in the profiling process

These improvements might have uncovered more tools if applied to the full dataset without filtering.
