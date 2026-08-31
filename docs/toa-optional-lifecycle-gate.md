# Optional TOA verify for MCP lifecycle / promote

Docker MCP Gateway manages MCP server lifecycle in containers, authenticates
clients, verifies Docker MCP image signatures for `mcp/` images, and can attach
interceptors to tool calls. Those controls answer isolation, auth, supply-chain
signatures, and optional request audit.

They do not answer whether a tool recently delivered a real result under an
outside probe.

[TOA](https://github.com/Carmel-Labs-Inc/toa) (`toa/0.1`) is an Apache-2.0 signed
JSON evidence format for MCP tool delivery (reach, invoke, functional, shape,
and related layers). It is not a wire protocol. It is not meant to run on every
live `tools/call`.

## Not the same as gateway audit attestation

This repository also discusses **audit attestation** for tamper-evident tool-call
records (see open design work around interceptors). That is a different artifact
from TOA:

| Concern | Docker MCP image signatures / audit attestation | TOA |
|---|---|---|
| What is signed | Image / call record integrity | Graded delivery evidence from a probe |
| When | Pull/run or per-call intercept | CI / promote / enable gate |
| Hot path | Gateway runtime | Offline `toa-verify` only |

## Suggested fit

Optional, off by default. Before enabling a server in a profile, pushing a
catalog/profile to an OCI registry, or promoting a Compose stack, require a
recent TOA attestation and verify it offline with `--require-emitter` and optional `--max-age`.

- Any party can emit if they sign the schema.
- AgentStatus is one optional emitter.
- No AgentStatus account is required to verify.

```yaml
      # After gateway health / compose smoke checks.
      - name: Verify tool delivery attestation
        if: hashFiles('toa.json') != ''
        run: |
          pip install "git+https://github.com/Carmel-Labs-Inc/toa.git@5a1bf1cf6a15a4864ea809fe7b2a073f2cef4e22#subdirectory=python"
          toa-verify toa.json --require-emitter agentstatus --require-layer functional=pass --max-age 7d
```

Copy-paste workflow: [`examples/toa-after-lifecycle.yml`](../examples/toa-after-lifecycle.yml).
Compose health example: [`examples/health`](../examples/health).

## Out of scope

- Replacing image signature verification, OAuth, or interceptors
- Signing every production `tools/call`
- Changing gateway runtime code

See also: [security model](security.md).
