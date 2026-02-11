import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StdioClientTransport } from "@modelcontextprotocol/sdk/client/stdio.js";

const transport = new StdioClientTransport({
  command: "/tmp/docker-mcp",
  args: ["gateway", "run", "--profile", "pypi-test-0"],
});

const client = new Client({ name: "quick-test", version: "1.0.0" });

console.log("Connecting...");
await client.connect(transport);
console.log("Connected! Listing tools...");

const result = await client.listTools();
console.log(`Found ${result.tools.length} tools:`);
for (const t of result.tools) {
  console.log(`  - ${t.name}: ${t.description?.slice(0, 80)}`);
}

await client.close();
console.log("Done.");
process.exit(0);
