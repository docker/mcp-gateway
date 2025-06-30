
group default {
  targets = [
    "all",
  ]
}

group all {
  targets = [
    "amcp-gateway",
    "jcat",
    "l4proxy",
    "l7proxy",
    "dns-forwarder",
  ]
}

# Required by docker/metadata-action and docker/bake-action gh actions.
target "docker-metadata-action" {}

target _base {
  inherits = ["docker-metadata-action"]
  output = ["type=docker"]
  platforms = ["linux/arm64", "linux/amd64"]
  attest = [
    {
      type = "provenance",
      mode = "max",
    },
    {
      type = "sbom",
    }
  ]
}

target jcat {
  inherits = ["_base"]
  context = "tools/jcat"
  output = ["type=image,name=docker/jcat"]
}


target l4proxy {
  inherits = ["_base"]
  context = "tools/l4proxy"
  output = ["type=image,name=docker/mcp-l4proxy:v1"]
}

target l7proxy {
  inherits = ["_base"]
  context = "tools/l7proxy"
  output = ["type=image,name=docker/mcp-l7proxy:v1"]
}

target dns-forwarder {
  inherits = ["_base"]
  context = "tools/dns-forwarder"
  output = ["type=image,name=docker/mcp-dns-forwarder:v1"]
}

target mcp-gateway {
  inherits = ["_base"]
  context = "."
  target = "mcp-gateway"
  output = [
    "type=image,name=docker/mcp-gateway",
    "type=image,name=docker/mcp-gateway:v1",
    "type=image,name=docker/agents_gateway:v2",
  ]
}

