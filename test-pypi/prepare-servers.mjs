#!/usr/bin/env node

/**
 * prepare-servers.mjs
 *
 * Fetches PyPI MCP server registry entries, deduplicates to latest version,
 * and uses the Claude CLI to analyze each server's requirements for testing.
 *
 * Usage:
 *   node prepare-servers.mjs [--force] [--concurrency N]
 *
 * Outputs:
 *   - test-pypi/servers/<serverName>.json  (per-server config)
 *   - test-pypi/server-manifest.json       (index of all servers)
 */

import { execSync } from "child_process";
import {
  readFileSync,
  writeFileSync,
  mkdirSync,
  existsSync,
  readdirSync,
} from "fs";
import { resolve, join } from "path";

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------
const SERVERS_FILE = resolve(import.meta.dirname, "../pypi-servers.txt");
const SERVERS_DIR = resolve(import.meta.dirname, "servers");
const MANIFEST_FILE = resolve(import.meta.dirname, "server-manifest.json");
const CONCURRENCY_DEFAULT = 1;

// ---------------------------------------------------------------------------
// CLI flags
// ---------------------------------------------------------------------------
const args = process.argv.slice(2);
const forceFlag = args.includes("--force");
const concurrencyIdx = args.indexOf("--concurrency");
const CONCURRENCY =
  concurrencyIdx !== -1
    ? parseInt(args[concurrencyIdx + 1], 10) || CONCURRENCY_DEFAULT
    : CONCURRENCY_DEFAULT;

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Replicate Go extractServerName: replace / and . with - */
function extractServerName(fullName) {
  return fullName.replace(/\//g, "-").replace(/\./g, "-");
}

/**
 * Parse a registry URL into { encodedName, decodedName, version }.
 * URL format: .../servers/<encoded-name>/versions/<version>
 */
function parseRegistryUrl(url) {
  const match = url.match(/servers\/([^/]+)\/versions\/(.+)$/);
  if (!match) return null;
  return {
    encodedName: match[1],
    decodedName: decodeURIComponent(match[1]),
    version: match[2],
  };
}

/**
 * Semver-aware comparison. Returns >0 if a > b, <0 if a < b, 0 if equal.
 * Falls back to string comparison for non-standard versions.
 */
function compareSemver(a, b) {
  const pa = a.split(".").map(Number);
  const pb = b.split(".").map(Number);
  for (let i = 0; i < Math.max(pa.length, pb.length); i++) {
    const va = pa[i] || 0;
    const vb = pb[i] || 0;
    if (isNaN(va) || isNaN(vb)) return a.localeCompare(b);
    if (va !== vb) return va - vb;
  }
  return 0;
}

/**
 * Fetch a URL using curl (avoids needing a fetch polyfill in Node < 18).
 * Returns the response body as a string.
 */
function fetchUrl(url) {
  try {
    const result = execSync(
      `curl -sS --max-time 30 --fail ${JSON.stringify(url)}`,
      { encoding: "utf-8", timeout: 35_000 }
    );
    return result;
  } catch (err) {
    throw new Error(
      `Failed to fetch ${url}: ${err.stderr?.toString() || err.message}`
    );
  }
}

// ---------------------------------------------------------------------------
// Step 1a: Load & Deduplicate
// ---------------------------------------------------------------------------

function loadAndDeduplicate() {
  const content = readFileSync(SERVERS_FILE, "utf-8");
  const urls = content
    .split("\n")
    .map((l) => l.trim())
    .filter((l) => l.startsWith("http"));

  console.log(`Loaded ${urls.length} URLs from pypi-servers.txt`);

  // Group by decoded server name, keep highest version
  const grouped = new Map(); // decodedName -> { url, version, decodedName, encodedName }
  for (const url of urls) {
    const parsed = parseRegistryUrl(url);
    if (!parsed) {
      console.warn(`  Skipping unparseable URL: ${url}`);
      continue;
    }
    const existing = grouped.get(parsed.decodedName);
    if (!existing || compareSemver(parsed.version, existing.version) > 0) {
      grouped.set(parsed.decodedName, { url, ...parsed });
    }
  }

  console.log(`Deduplicated to ${grouped.size} unique servers\n`);
  return [...grouped.values()];
}

// ---------------------------------------------------------------------------
// Step 1b: Claude Agent Analysis
// ---------------------------------------------------------------------------

const CLAUDE_PROMPT_TEMPLATE = `Given this MCP server registry entry, determine what's needed to test it
(listing tools via MCP protocol). Return ONLY valid JSON with this structure:

{
  "canTest": true/false,
  "skipReason": "...",
  "config": { "KEY": "value" },
  "services": [
    {
      "name": "service-name",
      "image": "image:tag",
      "ports": ["hostPort:containerPort"],
      "env": { "ENV_VAR": "value" }
    }
  ]
}

Rules:
- URLs to backing services must use host.docker.internal (e.g., bolt://host.docker.internal:7687)
- Only include config for non-secret env vars (API keys should NOT be guessed)
- If the server needs a paid/cloud/SaaS API key we can't provide, set canTest to false with skipReason explaining why
- If the server has no required env vars, return empty config {} and empty services []
- For services, use standard official Docker images
- Disable authentication on backing services where possible (test environment)
- canTest should be true if the server can work with only local backing services or no config at all
- config values should be actual usable values, not placeholders
- For database URLs, use host.docker.internal as the host

Registry entry:
`;

function analyzeWithClaude(registryJson) {
  const prompt = CLAUDE_PROMPT_TEMPLATE + JSON.stringify(registryJson, null, 2);

  try {
    const result = execSync(
      `claude -p ${JSON.stringify(prompt)} --output-format json`,
      {
        encoding: "utf-8",
        timeout: 120_000,
        maxBuffer: 10 * 1024 * 1024,
      }
    );

    // The claude CLI with --output-format json wraps the result.
    // Try to parse the outer JSON first, then extract the inner result.
    let parsed;
    try {
      parsed = JSON.parse(result);
    } catch {
      // If the output isn't valid JSON, try to extract JSON from the text
      const jsonMatch = result.match(/\{[\s\S]*\}/);
      if (jsonMatch) {
        parsed = JSON.parse(jsonMatch[0]);
      } else {
        throw new Error("No JSON found in Claude response");
      }
    }

    // claude --output-format json wraps in { result: "..." } or similar
    // Check if this is the wrapper or the actual analysis
    if (parsed.result && typeof parsed.result === "string") {
      // The inner result might be a JSON string
      try {
        const inner = JSON.parse(parsed.result);
        return normalizeAnalysis(inner);
      } catch {
        // Try extracting JSON from the result string
        const jsonMatch = parsed.result.match(/\{[\s\S]*\}/);
        if (jsonMatch) {
          return normalizeAnalysis(JSON.parse(jsonMatch[0]));
        }
        throw new Error("Could not parse inner JSON from Claude response");
      }
    }

    // Already the analysis object
    return normalizeAnalysis(parsed);
  } catch (err) {
    console.warn(`    Claude analysis failed: ${err.message}`);
    return {
      canTest: true,
      skipReason: "",
      config: {},
      services: [],
      analysisError: err.message,
    };
  }
}

function normalizeAnalysis(obj) {
  return {
    canTest: obj.canTest !== false,
    skipReason: obj.skipReason || "",
    config: obj.config && typeof obj.config === "object" ? obj.config : {},
    services: Array.isArray(obj.services) ? obj.services : [],
  };
}

// ---------------------------------------------------------------------------
// Step 1c: Process servers and save results
// ---------------------------------------------------------------------------

async function processServer(server, index, total) {
  const serverName = extractServerName(server.decodedName);
  const outputFile = join(SERVERS_DIR, `${serverName}.json`);

  // Cache check
  if (!forceFlag && existsSync(outputFile)) {
    process.stdout.write(
      `[${index + 1}/${total}] ${serverName} — cached, skipping\n`
    );
    return JSON.parse(readFileSync(outputFile, "utf-8"));
  }

  process.stdout.write(
    `[${index + 1}/${total}] ${serverName} — fetching registry... `
  );

  // Fetch registry JSON
  let registryJson;
  try {
    const body = fetchUrl(server.url);
    registryJson = JSON.parse(body);
  } catch (err) {
    console.log(`FETCH_FAILED - ${err.message.slice(0, 80)}`);
    const result = {
      url: server.url,
      serverName,
      registryName: server.decodedName,
      version: server.version,
      canTest: false,
      skipReason: `Failed to fetch registry: ${err.message.slice(0, 100)}`,
      config: {},
      services: [],
    };
    writeFileSync(outputFile, JSON.stringify(result, null, 2));
    return result;
  }

  process.stdout.write("analyzing with Claude... ");

  // Analyze with Claude
  const analysis = analyzeWithClaude(registryJson);

  const result = {
    url: server.url,
    serverName,
    registryName: server.decodedName,
    version: server.version,
    canTest: analysis.canTest,
    skipReason: analysis.skipReason,
    config: analysis.config,
    services: analysis.services,
  };

  if (analysis.analysisError) {
    result.analysisError = analysis.analysisError;
  }

  writeFileSync(outputFile, JSON.stringify(result, null, 2));

  if (!analysis.canTest) {
    console.log(`SKIP - ${analysis.skipReason.slice(0, 60)}`);
  } else if (analysis.services.length > 0) {
    console.log(
      `OK (needs: ${analysis.services.map((s) => s.name).join(", ")})`
    );
  } else if (Object.keys(analysis.config).length > 0) {
    console.log(`OK (config: ${Object.keys(analysis.config).join(", ")})`);
  } else {
    console.log("OK (no dependencies)");
  }

  return result;
}

async function main() {
  mkdirSync(SERVERS_DIR, { recursive: true });

  const servers = loadAndDeduplicate();

  console.log(`Processing servers (concurrency: ${CONCURRENCY})...\n`);

  const results = [];

  // Process sequentially or with low concurrency
  for (let i = 0; i < servers.length; i += CONCURRENCY) {
    const batch = servers.slice(i, i + CONCURRENCY);
    const batchResults = await Promise.all(
      batch.map((server, j) =>
        processServer(server, i + j, servers.length)
      )
    );
    results.push(...batchResults);
  }

  // Write manifest
  const manifest = {
    generatedAt: new Date().toISOString(),
    totalUrls: 310,
    uniqueServers: results.length,
    servers: results.map((r) => ({
      serverName: r.serverName,
      registryName: r.registryName,
      version: r.version,
      url: r.url,
      canTest: r.canTest,
    })),
  };
  writeFileSync(MANIFEST_FILE, JSON.stringify(manifest, null, 2));

  // Summary
  const testable = results.filter((r) => r.canTest);
  const withServices = testable.filter((r) => r.services.length > 0);
  const skipped = results.filter((r) => !r.canTest);

  console.log("\n" + "=".repeat(60));
  console.log("PREPARATION SUMMARY");
  console.log("=".repeat(60));
  console.log(`  Total unique servers:    ${results.length}`);
  console.log(`  Testable (no services):  ${testable.length - withServices.length}`);
  console.log(`  Testable (with services): ${withServices.length}`);
  console.log(`  Skipped (untestable):    ${skipped.length}`);
  console.log(`\nServer configs: ${SERVERS_DIR}/`);
  console.log(`Manifest:       ${MANIFEST_FILE}`);

  // List unique services needed
  const allServices = new Map();
  for (const r of results) {
    for (const svc of r.services) {
      allServices.set(svc.name, svc.image);
    }
  }
  if (allServices.size > 0) {
    console.log(`\nBacking services needed:`);
    for (const [name, image] of allServices) {
      console.log(`  ${name}: ${image}`);
    }
  }
}

main().catch((err) => {
  console.error("Fatal error:", err);
  process.exit(1);
});
