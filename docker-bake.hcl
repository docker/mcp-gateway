variable "GO_VERSION" {
  default = null
}

target "_common" {
  args = {
    GO_VERSION = GO_VERSION
    BUILDKIT_CONTEXT_KEEP_GIT_DIR = 1
  }
}

variable "DOCS_FORMATS" {
  default = "md,yaml"
}

variable "DOCS_FEATURES" {
  default = "profiles"
}

group default {
  targets = [
    "all",
  ]
}

group all {
  targets = [
    "mcp-gateway",
    "mcp-gateway-dind",
    "jcat",
    "l4proxy",
    "l7proxy",
    "dns-forwarder",
    "validate-docs",
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
    #"type=image,name=docker/mcp-gateway:v1", last commit pushed with v1 tag: 261fd774d271974ae196b1cbc3acc04aceb3257b 
    "type=image,name=docker/mcp-gateway:v2",
  ]
}

target mcp-gateway-dind {
  inherits = ["_base"]
  context = "."
  target = "mcp-gateway-dind"
  output = [
    "type=image,name=docker/mcp-gateway:dind",
  ]
}

target "validate-docs" {
  inherits = ["_common"]
  args = {
    DOCS_FORMATS   = DOCS_FORMATS
    DOCS_FEATURES  = DOCS_FEATURES
  }
  target = "docs-validate"
  output = ["type=cacheonly"]
}

target "update-docs" {
  inherits = ["_common"]
  args = {
    DOCS_FORMATS   = DOCS_FORMATS
    DOCS_FEATURES  = DOCS_FEATURES
  }
  target = "docs-update"
  output = ["./docs/generator/reference"]
}