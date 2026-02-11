import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StdioClientTransport } from "@modelcontextprotocol/sdk/client/stdio.js";
import { execSync, spawn } from "child_process";
import { readFileSync } from "fs";
import { resolve } from "path";

// Configuration
const DOCKER_BIN = "docker";
const SERVERS_FILE = resolve(import.meta.dirname, "../pypi-servers.txt");
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
  process.exit(1);
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

function loadServers() {
  const content = readFileSync(SERVERS_FILE, "utf-8");
  return content
    .split("\n")
    .map((line) => line.trim())
    .filter((line) => line.length > 0 && line.startsWith("http"));
}

function extractServerName(url) {
  // Extract a short name from the registry URL for display
  const match = url.match(/servers\/([^/]+)\/versions\/(.+)$/);
  if (match) {
    return `${decodeURIComponent(match[1])}@${match[2]}`;
  }
  return url;
}

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
        error: "Timeout after " + (timeoutMs / 1000) + "s",
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

async function testServer(serverUrl, index, total) {
  const name = extractServerName(serverUrl);
  const profileId = `pypi-test-${index}`;

  process.stdout.write(
    `[${index + 1}/${total}] Testing: ${name}... `
  );

  // Create profile
  const profileResult = await createProfile(serverUrl, profileId);
  if (!profileResult.ok) {
    console.log(`PROFILE_FAILED - ${profileResult.error?.slice(0, 100)}`);
    return {
      serverUrl,
      name,
      status: "profile_failed",
      error: profileResult.error,
      toolCount: 0,
      tools: [],
    };
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

  if (toolsResult.success && toolsResult.toolCount > 0) {
    console.log(
      `OK - ${toolsResult.toolCount} tools [${toolsResult.tools.slice(0, 3).join(", ")}${toolsResult.toolCount > 3 ? "..." : ""}]`
    );
    return {
      serverUrl,
      name,
      status: "tools_found",
      toolCount: toolsResult.toolCount,
      tools: toolsResult.tools,
    };
  } else if (toolsResult.success && toolsResult.toolCount === 0) {
    console.log(`NO_TOOLS - connected but 0 server tools`);
    return {
      serverUrl,
      name,
      status: "no_tools",
      toolCount: 0,
      tools: [],
    };
  } else {
    console.log(
      `FAILED - ${toolsResult.error?.slice(0, 100)}`
    );
    return {
      serverUrl,
      name,
      status: "failed",
      error: toolsResult.error,
      toolCount: 0,
      tools: [],
    };
  }
}

async function runBatch(servers, startIdx, total) {
  return Promise.all(
    servers.map((url, i) => testServer(url, startIdx + i, total))
  );
}

async function main() {
  const servers = loadServers();
  console.log(`\nLoaded ${servers.length} servers from pypi-servers.txt\n`);
  console.log("=".repeat(80));

  const results = [];
  const startTime = Date.now();

  // Process in batches for controlled concurrency
  for (let i = 0; i < servers.length; i += CONCURRENCY) {
    if (shuttingDown) break;
    const batch = servers.slice(i, i + CONCURRENCY);
    const batchResults = await runBatch(batch, i, servers.length);
    results.push(...batchResults);
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

  console.log(`\nTotal servers tested:     ${results.length}`);
  console.log(`Servers WITH tools:      ${toolsFound.length} (${((toolsFound.length / results.length) * 100).toFixed(1)}%)`);
  console.log(`Servers with NO tools:   ${noTools.length}`);
  console.log(`Servers FAILED:          ${failed.length}`);
  console.log(`Profile creation FAILED: ${profileFailed.length}`);
  console.log(`Time elapsed:            ${elapsed}s`);

  if (toolsFound.length > 0) {
    console.log(`\n--- Servers WITH tools (${toolsFound.length}) ---`);
    for (const r of toolsFound) {
      console.log(`  ${r.name}: ${r.toolCount} tools`);
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
    console.log(`\n--- Profile creation FAILED (${profileFailed.length}) ---`);
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
