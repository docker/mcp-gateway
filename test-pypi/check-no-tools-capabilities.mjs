import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StdioClientTransport } from "@modelcontextprotocol/sdk/client/stdio.js";
import { execSync } from "child_process";
import { readFileSync, writeFileSync, existsSync, readdirSync } from "fs";
import { resolve, join } from "path";
import {
  startBackingServices,
  stopBackingServices,
} from "./backing-services.mjs";

// Configuration
const DOCKER_BIN = "docker";
const RESULTS_FILE = resolve(import.meta.dirname, "results.json");
const SERVERS_DIR = resolve(import.meta.dirname, "servers");
const OUTPUT_FILE = resolve(
  import.meta.dirname,
  "no-tools-capabilities.json"
);
const TIMEOUT_MS = 120_000; // 2 minutes per server
const CONCURRENCY = 1; // how many servers to test in parallel

// Dynamic tools to exclude from counts (same as test-servers.mjs)
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
    try {
      transport._process?.kill("SIGKILL");
    } catch {}
  }
  activeTransports.clear();
  stopBackingServices()
    .catch(() => {})
    .finally(() => process.exit(1));
}

process.on("SIGINT", () => handleShutdown("SIGINT"));
process.on("SIGTERM", () => handleShutdown("SIGTERM"));

// ---------------------------------------------------------------------------
// Server config loading
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Capability testing
// ---------------------------------------------------------------------------

async function testServerCapabilities(profileId, timeoutMs) {
  return new Promise((resolvePromise) => {
    let resolved = false;
    let client;
    let transport;

    function finish(result) {
      if (resolved) return;
      resolved = true;
      clearTimeout(timer);

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
      });
    }, timeoutMs);

    (async () => {
      try {
        transport = new StdioClientTransport({
          command: DOCKER_BIN,
          args: ["mcp", "gateway", "run", "--profile", profileId],
          stderr: "pipe",
        });
        activeTransports.add(transport);

        client = new Client({ name: "capability-checker", version: "1.0.0" });

        await client.connect(transport);

        // Check all capabilities
        const [toolsResult, resourcesResult, promptsResult] =
          await Promise.all([
            client.listTools().catch((err) => ({ error: err.message })),
            client.listResources().catch((err) => ({ error: err.message })),
            client.listPrompts().catch((err) => ({ error: err.message })),
          ]);

        await client.close();

        const allTools = toolsResult.tools || [];
        const tools = allTools.filter((t) => !DYNAMIC_TOOLS.has(t.name));
        const resources = resourcesResult.resources || [];
        const prompts = promptsResult.prompts || [];

        finish({
          success: true,
          toolCount: tools.length,
          tools: tools.map((t) => t.name),
          resourceCount: resources.length,
          resources: resources.map((r) => r.uri),
          promptCount: prompts.length,
          prompts: prompts.map((p) => p.name),
          toolsError: toolsResult.error,
          resourcesError: resourcesResult.error,
          promptsError: promptsResult.error,
        });
      } catch (err) {
        finish({
          success: false,
          error: err.message,
        });
      }
    })();
  });
}

// ---------------------------------------------------------------------------
// Server test orchestration
// ---------------------------------------------------------------------------

async function testServer(serverInfo, index, total, serverConfigs) {
  const { serverUrl, name, serverName } = serverInfo;
  const profileId = `capability-test-${index}`;
  const config = serverConfigs.get(serverUrl);

  process.stdout.write(
    `[${index + 1}/${total}] Checking: ${name}... `
  );

  // Create profile
  const profileResult = await createProfile(serverUrl, profileId);
  if (!profileResult.ok) {
    console.log(`PROFILE_FAILED - ${profileResult.error?.slice(0, 100)}`);
    return {
      serverUrl,
      name,
      serverName,
      status: "profile_failed",
      error: profileResult.error,
    };
  }

  // Apply config if available
  if (config && serverName && config.config) {
    applyProfileConfig(profileId, serverName, config.config);
  }

  // Test capabilities
  const capResult = await testServerCapabilities(profileId, TIMEOUT_MS);

  // Clean up profile
  try {
    execSync(`${DOCKER_BIN} mcp profile remove ${profileId}`, {
      stdio: "pipe",
      timeout: 30_000,
    });
  } catch {
    // ignore cleanup errors
  }

  if (!capResult.success) {
    console.log(`FAILED - ${capResult.error?.slice(0, 100)}`);
    return {
      serverUrl,
      name,
      serverName,
      status: "failed",
      error: capResult.error,
    };
  }

  const hasResources = capResult.resourceCount > 0;
  const hasPrompts = capResult.promptCount > 0;
  const hasTools = capResult.toolCount > 0;

  const capabilities = [];
  if (hasTools) capabilities.push(`${capResult.toolCount} tools`);
  if (hasResources) capabilities.push(`${capResult.resourceCount} resources`);
  if (hasPrompts) capabilities.push(`${capResult.promptCount} prompts`);

  const capStr = capabilities.length > 0 ? capabilities.join(", ") : "nothing";
  console.log(`OK - ${capStr}`);

  return {
    serverUrl,
    name,
    serverName,
    status: "success",
    toolCount: capResult.toolCount,
    tools: capResult.tools,
    resourceCount: capResult.resourceCount,
    resources: capResult.resources,
    promptCount: capResult.promptCount,
    prompts: capResult.prompts,
    hasResources,
    hasPrompts,
    hasTools,
    errors: {
      tools: capResult.toolsError,
      resources: capResult.resourcesError,
      prompts: capResult.promptsError,
    },
  };
}

async function runBatch(servers, startIdx, total, serverConfigs) {
  return Promise.all(
    servers.map((info, i) =>
      testServer(info, startIdx + i, total, serverConfigs)
    )
  );
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

async function main() {
  // Load results.json
  if (!existsSync(RESULTS_FILE)) {
    console.error(`Error: ${RESULTS_FILE} not found`);
    console.error('Run "node test-servers.mjs" first to generate results.json');
    process.exit(1);
  }

  const allResults = JSON.parse(readFileSync(RESULTS_FILE, "utf-8"));
  console.log(`Loaded ${allResults.length} results from results.json`);

  // Filter for no_tools servers
  const noToolsServers = allResults.filter((r) => r.status === "no_tools");
  console.log(
    `Found ${noToolsServers.length} servers with "no_tools" status\n`
  );

  if (noToolsServers.length === 0) {
    console.log("No servers to check. Exiting.");
    return;
  }

  // Load server configs
  const serverConfigs = loadServerConfigs();
  const hasConfigs = serverConfigs.size > 0;

  if (hasConfigs) {
    console.log(`Loaded ${serverConfigs.size} server config(s) from servers/`);
  } else {
    console.log("No server configs found in servers/");
  }

  // Collect and start backing services
  let backingServicesStarted = false;
  if (hasConfigs) {
    const serversToTest = new Set(noToolsServers.map((s) => s.serverUrl));
    const allServices = [];
    for (const config of serverConfigs.values()) {
      if (
        serversToTest.has(config.url) &&
        Array.isArray(config.services)
      ) {
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
    for (let i = 0; i < noToolsServers.length; i += CONCURRENCY) {
      if (shuttingDown) break;
      const batch = noToolsServers.slice(i, i + CONCURRENCY);
      const batchResults = await runBatch(
        batch,
        i,
        noToolsServers.length,
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

  const successful = results.filter((r) => r.status === "success");
  const failed = results.filter((r) => r.status === "failed");
  const profileFailed = results.filter((r) => r.status === "profile_failed");

  // Categorize by what they have
  const hasResourcesOnly = successful.filter(
    (r) => r.hasResources && !r.hasPrompts && !r.hasTools
  );
  const hasPromptsOnly = successful.filter(
    (r) => r.hasPrompts && !r.hasResources && !r.hasTools
  );
  const hasResourcesAndPrompts = successful.filter(
    (r) => r.hasResources && r.hasPrompts && !r.hasTools
  );
  const hasNothing = successful.filter(
    (r) => !r.hasResources && !r.hasPrompts && !r.hasTools
  );
  const unexpectedTools = successful.filter((r) => r.hasTools);

  console.log(`\nTotal checked:           ${results.length}`);
  console.log(`Time elapsed:            ${elapsed}s`);

  console.log(`\n--- Results ---`);
  console.log(`  Successful:              ${successful.length}`);
  console.log(`  Failed:                  ${failed.length}`);
  console.log(`  Profile creation failed: ${profileFailed.length}`);

  console.log(`\n--- Breakdown of successful checks ---`);
  console.log(`  Resources only:          ${hasResourcesOnly.length}`);
  console.log(`  Prompts only:            ${hasPromptsOnly.length}`);
  console.log(`  Resources + Prompts:     ${hasResourcesAndPrompts.length}`);
  console.log(`  Nothing at all:          ${hasNothing.length}`);
  console.log(`  Unexpected tools:        ${unexpectedTools.length}`);

  if (hasResourcesOnly.length > 0) {
    console.log(`\n--- Servers with RESOURCES only (${hasResourcesOnly.length}) ---`);
    for (const r of hasResourcesOnly) {
      console.log(`  ${r.name}: ${r.resourceCount} resources`);
      if (r.resources.length <= 5) {
        console.log(`    ${r.resources.join(", ")}`);
      } else {
        console.log(
          `    ${r.resources.slice(0, 5).join(", ")} ... and ${r.resources.length - 5} more`
        );
      }
    }
  }

  if (hasPromptsOnly.length > 0) {
    console.log(`\n--- Servers with PROMPTS only (${hasPromptsOnly.length}) ---`);
    for (const r of hasPromptsOnly) {
      console.log(`  ${r.name}: ${r.promptCount} prompts`);
      if (r.prompts.length <= 5) {
        console.log(`    ${r.prompts.join(", ")}`);
      } else {
        console.log(
          `    ${r.prompts.slice(0, 5).join(", ")} ... and ${r.prompts.length - 5} more`
        );
      }
    }
  }

  if (hasResourcesAndPrompts.length > 0) {
    console.log(
      `\n--- Servers with RESOURCES + PROMPTS (${hasResourcesAndPrompts.length}) ---`
    );
    for (const r of hasResourcesAndPrompts) {
      console.log(
        `  ${r.name}: ${r.resourceCount} resources, ${r.promptCount} prompts`
      );
    }
  }

  if (hasNothing.length > 0) {
    console.log(`\n--- Servers with NOTHING (${hasNothing.length}) ---`);
    for (const r of hasNothing) {
      console.log(`  ${r.name}`);
    }
  }

  if (unexpectedTools.length > 0) {
    console.log(
      `\n--- Servers with UNEXPECTED tools (${unexpectedTools.length}) ---`
    );
    for (const r of unexpectedTools) {
      console.log(`  ${r.name}: ${r.toolCount} tools`);
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
  writeFileSync(OUTPUT_FILE, JSON.stringify(results, null, 2));
  console.log(`\nDetailed results written to: ${OUTPUT_FILE}`);
}

main().catch((err) => {
  console.error("Fatal error:", err);
  process.exit(1);
});
