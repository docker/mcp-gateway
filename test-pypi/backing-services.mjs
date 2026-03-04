/**
 * backing-services.mjs
 *
 * Manages Docker backing services (databases, queues, etc.) needed by
 * PyPI MCP servers during testing. Reads service definitions from
 * per-server config files — nothing is hardcoded.
 *
 * Exports:
 *   startBackingServices(serviceList) — start deduplicated containers
 *   stopBackingServices()             — stop and remove all pypi-test-* containers
 */

import { execSync } from "child_process";
import { setTimeout as sleep } from "timers/promises";
import net from "net";

const DOCKER_BIN = "docker";
const CONTAINER_PREFIX = "pypi-test-";
const PORT_READY_TIMEOUT_MS = 60_000;
const PORT_CHECK_INTERVAL_MS = 2_000;

/**
 * Start backing service containers.
 *
 * @param {Array<{name: string, image: string, ports: string[], env: object}>} serviceList
 *   Collected from all server configs. Deduplicated by `name`.
 * @returns {Map<string, {name: string, containerId: string}>} running services
 */
export async function startBackingServices(serviceList) {
  if (!serviceList || serviceList.length === 0) {
    return new Map();
  }

  // Deduplicate by service name (first definition wins)
  const unique = new Map();
  for (const svc of serviceList) {
    if (!unique.has(svc.name)) {
      unique.set(svc.name, svc);
    }
  }

  console.log(`\nStarting ${unique.size} backing service(s)...`);

  const running = new Map();

  for (const [name, svc] of unique) {
    const containerName = `${CONTAINER_PREFIX}${name}`;

    // Remove existing container if present
    try {
      execSync(`${DOCKER_BIN} rm -f ${containerName}`, {
        stdio: "pipe",
        timeout: 15_000,
      });
    } catch {
      // Container didn't exist — fine
    }

    // Build docker run command
    const cmdParts = [
      DOCKER_BIN,
      "run",
      "-d",
      "--name",
      containerName,
    ];

    // Port mappings
    if (Array.isArray(svc.ports)) {
      for (const port of svc.ports) {
        cmdParts.push("-p", port);
      }
    }

    // Environment variables
    if (svc.env && typeof svc.env === "object") {
      for (const [key, value] of Object.entries(svc.env)) {
        cmdParts.push("-e", `${key}=${value}`);
      }
    }

    cmdParts.push(svc.image);

    const cmd = cmdParts
      .map((p) => (p.includes(" ") ? `"${p}"` : p))
      .join(" ");

    process.stdout.write(`  ${name} (${svc.image})... `);

    try {
      const containerId = execSync(cmd, {
        encoding: "utf-8",
        timeout: 60_000,
        stdio: ["pipe", "pipe", "pipe"],
      }).trim();

      running.set(name, { name, containerId: containerId.slice(0, 12) });

      // Wait for ports to be ready
      if (Array.isArray(svc.ports) && svc.ports.length > 0) {
        const hostPorts = svc.ports.map((p) => {
          const parts = p.split(":");
          return parseInt(parts[0], 10);
        });

        const ready = await waitForPorts(hostPorts, PORT_READY_TIMEOUT_MS);
        if (ready) {
          console.log(`started (${containerId.slice(0, 12)})`);
        } else {
          console.log(
            `started (${containerId.slice(0, 12)}) — port not ready within timeout, continuing anyway`
          );
        }
      } else {
        console.log(`started (${containerId.slice(0, 12)})`);
      }
    } catch (err) {
      console.log(
        `FAILED — ${(err.stderr?.toString() || err.message).slice(0, 100)}`
      );
    }
  }

  console.log("");
  return running;
}

/**
 * Stop and remove all pypi-test-* containers.
 */
export async function stopBackingServices() {
  try {
    const ps = execSync(
      `${DOCKER_BIN} ps -a --filter "name=${CONTAINER_PREFIX}" --format "{{.Names}}"`,
      { encoding: "utf-8", timeout: 15_000 }
    ).trim();

    if (!ps) return;

    const containers = ps.split("\n").filter(Boolean);
    if (containers.length === 0) return;

    console.log(
      `\nStopping ${containers.length} backing service container(s)...`
    );

    for (const name of containers) {
      try {
        execSync(`${DOCKER_BIN} rm -f ${name}`, {
          stdio: "pipe",
          timeout: 15_000,
        });
        console.log(`  Removed ${name}`);
      } catch {
        console.log(`  Failed to remove ${name}`);
      }
    }
  } catch (err) {
    console.warn(
      `Warning: could not list backing service containers: ${err.message}`
    );
  }
}

/**
 * Wait for TCP ports to accept connections on localhost.
 * Returns true if all ports are ready, false on timeout.
 */
async function waitForPorts(ports, timeoutMs) {
  const deadline = Date.now() + timeoutMs;

  for (const port of ports) {
    while (Date.now() < deadline) {
      if (await checkPort(port)) break;
      await sleep(PORT_CHECK_INTERVAL_MS);
    }
    if (Date.now() >= deadline) return false;
  }

  return true;
}

function checkPort(port) {
  return new Promise((resolve) => {
    const socket = new net.Socket();
    socket.setTimeout(1000);
    socket.once("connect", () => {
      socket.destroy();
      resolve(true);
    });
    socket.once("error", () => {
      socket.destroy();
      resolve(false);
    });
    socket.once("timeout", () => {
      socket.destroy();
      resolve(false);
    });
    socket.connect(port, "127.0.0.1");
  });
}
