package oauth

import "os"

// IsCEMode returns true if running in Docker CE mode (standalone OAuth flows).
// When false, uses Docker Desktop for OAuth orchestration.
//
// Set the environment variable DOCKER_MCP_USE_CE=true to enable CE mode.
func IsCEMode() bool {
	return os.Getenv("DOCKER_MCP_USE_CE") == "true"
}
