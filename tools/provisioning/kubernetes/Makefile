# Makefile for MCP Gateway Kubernetes provisioning and management tools

.PHONY: all clean cluster-tools install run

# Default target
all: cluster-tools

# Build the MCP cluster provisioning tool (renamed from cleanup)
cluster-tools: cluster-tools.go
	go build -o cluster-tools cluster-tools.go

# Legacy targets for backwards compatibility
mcp-tool: cluster-tools
	ln -sf cluster-tools mcp-tool

cleanup: cluster-tools
	ln -sf cluster-tools cleanup

# Install tools to /usr/local/bin (requires sudo)
install: cluster-tools
	sudo cp cluster-tools /usr/local/bin/cluster-tools
	@echo "Installed as 'cluster-tools'"

# Clean up built binaries
clean:
	rm -f cluster-tools mcp-tool cleanup

# Build and run the cluster tools with arguments (builds to temp directory)
# Usage: make run list
# Usage: make run session mcp-gateway-abc123
# Usage: make run create-configmap my-config '{"KEY":"value"}' default
# Usage: make run show-example firewalla-mcp-server
run:
	go build -o /tmp/cluster-tools cluster-tools.go
	/tmp/cluster-tools $(filter-out $@,$(MAKECMDGOALS))
	rm -f /tmp/cluster-tools

# Treat any argument after 'run' as a target (but don't do anything with them)
%:
	@:

# Test the build
test:
	go build -o /tmp/cluster-tools-test cluster-tools.go && rm /tmp/cluster-tools-test
	@echo "Build test successful"

# Show help
help:
	@echo "Available targets:"
	@echo "  all           - Build all tools (default)"
	@echo "  cluster-tools - Build the MCP cluster provisioning tool"
	@echo "  mcp-tool      - Legacy symlink to cluster-tools"
	@echo "  cleanup       - Legacy symlink to cluster-tools"
	@echo "  run           - Build and run cluster tools with ARGS"
	@echo "  install       - Install tools to /usr/local/bin"
	@echo "  clean         - Remove built binaries"
	@echo "  test          - Test build without creating files"
	@echo "  help          - Show this help"
	@echo ""
	@echo "Examples:"
	@echo "  make run list"
	@echo "  make run session mcp-gateway-abc123"
	@echo "  make run stale 24h default"
	@echo "  make run create-configmap my-config '{\"KEY\":\"value\"}' default"
	@echo "  make run create-secret my-secrets '{\"TOKEN\":\"secret\"}' default"
	@echo "  make run show-example firewalla-mcp-server"