// Copyright 2025-2026 Docker, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package api

import (
	"os"
	"path/filepath"
	"time"
)

const (
	// PluginLaunchedByEngineVar is used to inform engine-launched plugins about their name.
	PluginLaunchedByEngineVar = "DOCKER_SECRETS_ENGINE_PLUGIN_LAUNCH_CFG"
	// DefaultPluginRegistrationTimeout is the default timeout for plugin registration.
	DefaultPluginRegistrationTimeout = 5 * time.Second
	// DefaultClientRequestTimeout is the default timeout for clients to handle a request.
	DefaultClientRequestTimeout = time.Duration(0) // 0 means no limit
	// DefaultClientResponseHeaderTimeout is the default timeout for clients to handle
	// header responses, this does not include the response body and usually should
	// be short.
	DefaultClientResponseHeaderTimeout = time.Second
	// DefaultClientTLSHandshakeTimeout is the default timeout for clients to handle
	// tls handshakes. It should usually be short.
	DefaultClientTLSHandshakeTimeout = time.Second
	// DefaultClientIdleConnTimeout is the time a connection may stay alive for.
	// Clients that are long-lived take advantage of re-using the same connection
	// when making subsequent requests. This reduces latency.
	DefaultClientIdleConnTimeout = 90 * time.Second
	// DefaultClientMaxConnsPerHost is the maximum number of open connections
	// to the same host.
	DefaultClientMaxConnsPerHost = 100
	// DefaultClientMaxIdleConnsPerHost is the maximum number of idle connections
	// to the same host. Long-lived clients can re-use a connection from the
	// connection pool.
	DefaultClientMaxIdleConnsPerHost = 10
)

func DefaultSocketPath() string {
	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, "docker-secrets-engine", "engine.sock")
	}
	return filepath.Join(os.TempDir(), "docker-secrets-engine", "engine.sock")
}

func DefaultSecretsEngineDirectory() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "docker-secrets-engine"), nil
}
