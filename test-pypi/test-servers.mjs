import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StdioClientTransport } from "@modelcontextprotocol/sdk/client/stdio.js";
import { execSync, spawn } from "child_process";
import { readFileSync, existsSync, readdirSync } from "fs";
import { resolve, join } from "path";
import {
  startBackingServices,
  stopBackingServices,
} from "./backing-services.mjs";

// Configuration
const DOCKER_BIN = "docker";
const SERVERS_FILE = resolve(import.meta.dirname, "../pypi-servers.txt");
const SERVERS_DIR = resolve(import.meta.dirname, "servers");
const MANIFEST_FILE = resolve(import.meta.dirname, "server-manifest.json");
const TIMEOUT_MS = 120_000; // 2 minutes per server
const CONCURRENCY = 1; // how many servers to test in parallel

// Track active child processes for cleanup on Ctrl+C
const activeTransports = new Set();
let shuttingDown = false;

function handleShutdown(signal) {
  if (shuttingDown) return;
  shuttingDown = true;
  console.log(`\n\nReceived ${signal}, killing child processes...`);
  for (const transport of activeTransports) {
    try {
      transport.close?.();
    } catch {}
    // Also try to kill the underlying process directly
    try {
      transport._process?.kill("SIGKILL");
    } catch {}
  }
  activeTransports.clear();
  // Stop backing services before exiting
  stopBackingServices()
    .catch(() => {})
    .finally(() => process.exit(1));
}

process.on("SIGINT", () => handleShutdown("SIGINT"));
process.on("SIGTERM", () => handleShutdown("SIGTERM"));

// Dynamic tools to exclude from counts
const DYNAMIC_TOOLS = new Set([
  "mcp-find",
  "mcp-add",
  "mcp-remove",
  "mcp-config-set",
  "code-mode",
  "mcp-exec",
  "mcp-create-profile",
  "mcp-activate-profile",
  "find-tools",
  "mcp-discover",
]);

// ---------------------------------------------------------------------------
// Server config loading
// ---------------------------------------------------------------------------

/**
 * Load per-server configs from test-pypi/servers/*.json.
 * Returns a Map keyed by registry URL.
 */
function loadServerConfigs() {
  const configs = new Map();
  if (!existsSync(SERVERS_DIR)) return configs;

  const files = readdirSync(SERVERS_DIR).filter((f) => f.endsWith(".json"));
  for (const file of files) {
    try {
      const data = JSON.parse(
        readFileSync(join(SERVERS_DIR, file), "utf-8")
      );
      if (data.url) {
        configs.set(data.url, data);
      }
    } catch {
      // Skip malformed configs
    }
  }
  return configs;
}

/**
 * Load server URLs. Prefers manifest (deduplicated), falls back to raw file.
 */
function loadServerUrls() {
  // Prefer the deduplicated manifest
  if (existsSync(MANIFEST_FILE)) {
    try {
      const manifest = JSON.parse(readFileSync(MANIFEST_FILE, "utf-8"));
      const urls = manifest.servers.map((s) => s.url);
      if (urls.length > 0) {
        console.log(
          `Loaded ${urls.length} deduplicated servers from server-manifest.json`
        );
        return urls;
      }
    } catch {
      // Fall through to raw file
    }
  }

  // Fall back to raw pypi-servers.txt
  const content = readFileSync(SERVERS_FILE, "utf-8");
  const urls = content
    .split("\n")
    .map((line) => line.trim())
    .filter((line) => line.length > 0 && line.startsWith("http"));
  console.log(`Loaded ${urls.length} servers from pypi-servers.txt`);
  return urls;
}

function extractServerDisplayName(url) {
  // Extract a short name from the registry URL for display
  const match = url.match(/servers\/([^/]+)\/versions\/(.+)$/);
  if (match) {
    return `${decodeURIComponent(match[1])}@${match[2]}`;
  }
  return url;
}

// ---------------------------------------------------------------------------
// Profile management
// ---------------------------------------------------------------------------

async function createProfile(serverUrl, profileId) {
  try {
    // Remove profile if it exists (ignore errors)
    try {
      execSync(`${DOCKER_BIN} mcp profile remove ${profileId}`, {
        stdio: "pipe",
        timeout: 30_000,
      });
    } catch {
      // Profile might not exist, that's fine
    }

    // Create profile with the server
    execSync(
      `${DOCKER_BIN} mcp profile create --name "${profileId}" --id "${profileId}" --server "${serverUrl}"`,
      { stdio: "pipe", timeout: 60_000 }
    );
    return { ok: true };
  } catch (err) {
    return { ok: false, error: err.stderr?.toString() || err.message };
  }
}

/**
 * Apply config values to a profile.
 * Uses: docker mcp profile config <profileId> --set <serverName>.<key>=<value>
 */
function applyProfileConfig(profileId, serverName, config) {
  if (!config || Object.keys(config).length === 0) return;

  for (const [key, value] of Object.entries(config)) {
    const setValue =
      typeof value === "string" ? value : JSON.stringify(value);
    const arg = `${serverName}.${key}=${setValue}`;
    try {
      execSync(
        `${DOCKER_BIN} mcp profile config ${profileId} --set ${JSON.stringify(arg)}`,
        { stdio: "pipe", timeout: 15_000 }
      );
    } catch (err) {
      console.warn(
        `    Warning: failed to set config ${key}: ${(err.stderr?.toString() || err.message).slice(0, 80)}`
      );
    }
  }
}

// ---------------------------------------------------------------------------
// Tool testing
// ---------------------------------------------------------------------------

async function testServerTools(profileId, timeoutMs) {
  return new Promise((resolvePromise) => {
    let resolved = false;
    let client;
    let transport;

    function finish(result) {
      if (resolved) return;
      resolved = true;
      clearTimeout(timer);

      // Kill the gateway process to ensure cleanup
      try {
        transport?.close?.();
      } catch {}
      try {
        transport?._process?.kill("SIGKILL");
      } catch {}
      try {
        client?.close?.();
      } catch {}
      if (transport) activeTransports.delete(transport);

      resolvePromise(result);
    }

    const timer = setTimeout(() => {
      finish({
        success: false,
        error: "Timeout after " + timeoutMs / 1000 + "s",
        toolCount: 0,
        tools: [],
      });
    }, timeoutMs);

    (async () => {
      try {
        transport = new StdioClientTransport({
          command: DOCKER_BIN,
          args: ["mcp", "gateway", "run", "--profile", profileId],
          stderr: "pipe", // don't pollute console with gateway logs
        });
        activeTransports.add(transport);

        client = new Client({ name: "pypi-tester", version: "1.0.0" });

        await client.connect(transport);

        const result = await client.listTools();
        const allTools = result.tools || [];

        await client.close();

        // Filter out dynamic/default tools
        const serverTools = allTools.filter(
          (t) => !DYNAMIC_TOOLS.has(t.name)
        );

        finish({
          success: true,
          toolCount: serverTools.length,
          tools: serverTools.map((t) => t.name),
          dynamicToolCount: allTools.length - serverTools.length,
        });
      } catch (err) {
        finish({
          success: false,
          error: err.message,
          toolCount: 0,
          tools: [],
        });
      }
    })();
  });
}

// ---------------------------------------------------------------------------
// Server test orchestration
// ---------------------------------------------------------------------------

async function testServer(serverUrl, index, total, serverConfigs) {
  const displayName = extractServerDisplayName(serverUrl);
  const profileId = `pypi-test-${index}`;
  const config = serverConfigs.get(serverUrl);

  // Check if server is marked as untestable
  if (config && config.canTest === false) {
    console.log(
      `[${index + 1}/${total}] SKIPPED: ${displayName} — ${config.skipReason || "untestable"}`
    );
    return {
      serverUrl,
      name: displayName,
      serverName: config?.serverName,
      status: "skipped",
      skipReason: config.skipReason || "untestable",
      toolCount: 0,
      tools: [],
      hasServices: false,
    };
  }

  process.stdout.write(`[${index + 1}/${total}] Testing: ${displayName}... `);

  // Create profile
  const profileResult = await createProfile(serverUrl, profileId);
  if (!profileResult.ok) {
    console.log(`PROFILE_FAILED - ${profileResult.error?.slice(0, 100)}`);
    return {
      serverUrl,
      name: displayName,
      serverName: config?.serverName,
      status: "profile_failed",
      error: profileResult.error,
      toolCount: 0,
      tools: [],
      hasServices: !!(config?.services?.length),
    };
  }

  // Apply config if available
  if (config && config.serverName && config.config) {
    applyProfileConfig(profileId, config.serverName, config.config);
  }

  // Test tools
  const toolsResult = await testServerTools(profileId, TIMEOUT_MS);

  // Clean up profile
  try {
    execSync(`${DOCKER_BIN} mcp profile remove ${profileId}`, {
      stdio: "pipe",
      timeout: 30_000,
    });
  } catch {
    // ignore cleanup errors
  }

  const hasServices = !!(config?.services?.length);

  if (toolsResult.success && toolsResult.toolCount > 0) {
    console.log(
      `OK - ${toolsResult.toolCount} tools [${toolsResult.tools.slice(0, 3).join(", ")}${toolsResult.toolCount > 3 ? "..." : ""}]`
    );
    return {
      serverUrl,
      name: displayName,
      serverName: config?.serverName,
      status: "tools_found",
      toolCount: toolsResult.toolCount,
      tools: toolsResult.tools,
      hasServices,
    };
  } else if (toolsResult.success && toolsResult.toolCount === 0) {
    console.log(`NO_TOOLS - connected but 0 server tools`);
    return {
      serverUrl,
      name: displayName,
      serverName: config?.serverName,
      status: "no_tools",
      toolCount: 0,
      tools: [],
      hasServices,
    };
  } else {
    console.log(`FAILED - ${toolsResult.error?.slice(0, 100)}`);
    return {
      serverUrl,
      name: displayName,
      serverName: config?.serverName,
      status: "failed",
      error: toolsResult.error,
      toolCount: 0,
      tools: [],
      hasServices,
    };
  }
}

async function runBatch(servers, startIdx, total, serverConfigs) {
  return Promise.all(
    servers.map((url, i) =>
      testServer(url, startIdx + i, total, serverConfigs)
    )
  );
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

async function main() {
  // Load server configs (from prepare-servers.mjs output)
  const serverConfigs = loadServerConfigs();
  const hasConfigs = serverConfigs.size > 0;

  if (hasConfigs) {
    console.log(`Loaded ${serverConfigs.size} server config(s) from servers/`);
  } else {
    console.log(
      "No server configs found in servers/ — running without config/service support"
    );
    console.log(
      '  (Run "node prepare-servers.mjs" first for full support)\n'
    );
  }

  // Load server URLs
  const servers = loadServerUrls();

  // Collect and start backing services
  let backingServicesStarted = false;
  if (hasConfigs) {
    const allServices = [];
    for (const config of serverConfigs.values()) {
      if (config.canTest !== false && Array.isArray(config.services)) {
        allServices.push(...config.services);
      }
    }

    if (allServices.length > 0) {
      await startBackingServices(allServices);
      backingServicesStarted = true;
    }
  }

  console.log("\n" + "=".repeat(80));

  const results = [];
  const startTime = Date.now();

  try {
    // Process in batches for controlled concurrency
    for (let i = 0; i < servers.length; i += CONCURRENCY) {
      if (shuttingDown) break;
      const batch = servers.slice(i, i + CONCURRENCY);
      const batchResults = await runBatch(
        batch,
        i,
        servers.length,
        serverConfigs
      );
      results.push(...batchResults);
    }
  } finally {
    // Always clean up backing services
    if (backingServicesStarted) {
      await stopBackingServices();
    }
  }

  const elapsed = ((Date.now() - startTime) / 1000).toFixed(1);

  // Generate report
  console.log("\n" + "=".repeat(80));
  console.log("REPORT");
  console.log("=".repeat(80));

  const toolsFound = results.filter((r) => r.status === "tools_found");
  const noTools = results.filter((r) => r.status === "no_tools");
  const failed = results.filter((r) => r.status === "failed");
  const profileFailed = results.filter((r) => r.status === "profile_failed");
  const skipped = results.filter((r) => r.status === "skipped");

  // Enhanced reporting: split by service dependency
  const toolsNoSvc = toolsFound.filter((r) => !r.hasServices);
  const toolsWithSvc = toolsFound.filter((r) => r.hasServices);

  const tested = results.filter((r) => r.status !== "skipped");

  console.log(`\nTotal servers:           ${results.length}`);
  console.log(`Tested:                  ${tested.length}`);
  console.log(`Skipped (untestable):    ${skipped.length}`);
  console.log(`Time elapsed:            ${elapsed}s`);

  console.log(`\n--- Results ---`);
  if (toolsWithSvc.length > 0 || toolsNoSvc.length > 0) {
    console.log(
      `  Testable (no services):   ${toolsNoSvc.length}/${tested.length} with tools`
    );
    console.log(
      `  Testable (with services): ${toolsWithSvc.length}/${tested.length} with tools`
    );
  } else {
    console.log(
      `  Servers WITH tools:       ${toolsFound.length} (${((toolsFound.length / Math.max(tested.length, 1)) * 100).toFixed(1)}%)`
    );
  }
  console.log(`  Servers with NO tools:    ${noTools.length}`);
  console.log(`  Servers FAILED:           ${failed.length}`);
  console.log(`  Profile creation FAILED:  ${profileFailed.length}`);

  if (skipped.length > 0) {
    console.log(`\n--- Skipped servers (${skipped.length}) ---`);
    for (const r of skipped) {
      console.log(`  ${r.name}: ${r.skipReason}`);
    }
  }

  if (toolsFound.length > 0) {
    console.log(`\n--- Servers WITH tools (${toolsFound.length}) ---`);
    for (const r of toolsFound) {
      const svcTag = r.hasServices ? " [svc]" : "";
      console.log(`  ${r.name}: ${r.toolCount} tools${svcTag}`);
      if (r.tools.length <= 10) {
        console.log(`    Tools: ${r.tools.join(", ")}`);
      } else {
        console.log(
          `    Tools: ${r.tools.slice(0, 10).join(", ")} ... and ${r.tools.length - 10} more`
        );
      }
    }
  }

  if (noTools.length > 0) {
    console.log(`\n--- Servers with NO tools (${noTools.length}) ---`);
    for (const r of noTools) {
      console.log(`  ${r.name}`);
    }
  }

  if (failed.length > 0) {
    console.log(`\n--- FAILED servers (${failed.length}) ---`);
    for (const r of failed) {
      console.log(`  ${r.name}: ${r.error?.slice(0, 120)}`);
    }
  }

  if (profileFailed.length > 0) {
    console.log(
      `\n--- Profile creation FAILED (${profileFailed.length}) ---`
    );
    for (const r of profileFailed) {
      console.log(`  ${r.name}: ${r.error?.slice(0, 120)}`);
    }
  }

  // Write JSON results
  const reportPath = resolve(import.meta.dirname, "results.json");
  const { writeFileSync } = await import("fs");
  writeFileSync(reportPath, JSON.stringify(results, null, 2));
  console.log(`\nDetailed results written to: ${reportPath}`);
}

main().catch((err) => {
  console.error("Fatal error:", err);
  process.exit(1);
});
