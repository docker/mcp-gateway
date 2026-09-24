# Per-server container resource limits

MCP servers have different resource needs. A code analysis server may need more
CPU than a small API adapter. The gateway supports per-server CPU and memory
limits for the MCP server containers it launches.

Existing catalogs keep their behavior: `--cpus` (default `1`) and `--memory`
(default `2Gb`) apply to each server unless a per-server limit is set. These are
per-container limits, not reservations or a shared budget for the gateway.

## Catalog limits

A container server entry can declare an optional `resources` block:

```yaml
name: api-adapter
type: server
image: example/api-adapter:latest
resources:
  cpus: "0.25"
  memory: "256m"
```

The same block works inside a profile's `snapshot.server`. It is preserved by
JSON/YAML serialization of catalog entries and profile snapshots. It is not a
server environment variable or a field under the server's `config` schema.

Each omitted field inherits its global setting. CPU limits support decimal
values, with a minimum of `0.01` and nanocpu precision (up to nine decimal
places). Quote CPU values in YAML. Memory uses Docker size syntax, such as
`256m`, `2g`, or `512MiB`, with a minimum of 6 MiB. Per-server limits must be
positive; zero does not disable a limit.

## Operator overrides

Use repeatable flags to explicitly set a server's limits, including increases:

```console
docker mcp gateway run --profile development \
  --server-cpus code-analysis=2 \
  --server-memory code-analysis=4g \
  --server-cpus api-adapter=0.25 \
  --server-memory api-adapter=256m
```

Use the server name in the profile/catalog, not an image reference or a tool
name. Overrides also apply when that server is added dynamically later in the
session. Names do not have to be enabled at startup. Names are matched exactly;
check spelling when configuring an override. Repeating the same name for the
same flag uses the last value.

Precedence is evaluated independently for CPU and memory:

1. An explicit `--server-cpus name=value` or `--server-memory name=value` wins.
2. Otherwise, a catalog value applies only if it does not exceed the global
   limit for that resource.
3. Otherwise, the global setting applies.

A catalog request exceeding a global limit is rejected with the server name and
the explicit override needed to authorize it. It is not silently clamped. This
prevents imported catalog/profile data from increasing resource limits without
an operator decision. Raising a server override does not raise other servers'
limits. An override also lets an operator replace an invalid catalog value.

The existing global opt-outs (`--cpus=0`, `--memory=0` or an empty memory setting)
remain unchanged. A catalog can impose a positive limit where the global setting
is unlimited.

Operator overrides are validated before gateway startup I/O. Catalog values for
active container servers are validated before image pulls, on configuration
reload, and before client acquisition. Invalid dynamic additions fail before
their image is pulled. Existing servers without `resources` retain their current
container arguments.

## Scope

These limits apply to both short-lived and long-lived containerized MCP servers.
They do not allocate resources to remote HTTP/SSE servers, containers managed
externally with `--static`, the gateway itself, proxy sidecars, or legacy
per-tool POCI containers. Those keep their existing behavior. Updating launch
flags requires restarting the gateway; this feature does not resize running
containers in place.

## Verify with Docker

A Docker-backed integration test checks the actual container `HostConfig` values.
Supply a locally available image that includes `sleep`; the test never pulls it:

```console
MCP_RESOURCE_TEST_IMAGE=alpine:3.22 go test ./pkg/gateway \
  -run '^TestIntegrationServerResourceLimits$' -count=1
```

Without `MCP_RESOURCE_TEST_IMAGE`, or with `go test -short`, this integration test
is skipped. Unit tests cover limit precedence, validation, catalog round trips,
container arguments, and rejection before Docker access.
