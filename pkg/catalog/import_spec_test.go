package catalog

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestImportedServer_DropsRuntimeFields documents the field allowlist for
// imported sources: parsing a payload that names every runtime-shaping
// field should produce a Server with all of those fields zero-valued.
func TestImportedServer_DropsRuntimeFields(t *testing.T) {
	payload := []byte(`
name: imported-name
type: server
image: imported-image
description: imported description
title: Imported Title
icon: imported.png
readme: https://example.com/readme

# Runtime-shaping fields that must NOT survive an import.
command: ["/bin/sh", "-c", "echo hello"]
volumes:
  - /data:/data
  - /var/run/docker.sock:/var/run/docker.sock
user: root
extraHosts:
  - "extra.example:127.0.0.1"
allowHosts:
  - "*"
disableNetwork: true
longLived: true
sseEndpoint: https://remote.example/sse
remote:
  url: https://remote.example
  transport_type: sse
  headers:
    X-Custom: yes
oauth:
  providers:
    - provider: example-provider
      env: PROVIDER_TOKEN
  scopes:
    - "*"

# Env values are not recognised from imports; only names survive.
env:
  - name: LOG_LEVEL
  - name: DOCKER_HOST
    value: tcp://other-host:2375

# Secrets are not recognised from imports; only catalogs can bind secrets.
secrets:
  - name: api-key
    env: API_KEY

# Tool container config is not recognised from imports.
tools:
  - name: list_things
    description: lists things
    container:
      image: example/image
      command: ["/bin/sh"]
      volumes: ["/data:/data"]
      user: root

prefix: ok
config:
  - name: my-config
    type: object
metadata:
  category: utility
  tags: ["community"]
`)

	var imported ImportedServer
	require.NoError(t, yaml.Unmarshal(payload, &imported))
	srv := imported.ToServer()

	t.Run("descriptive fields survive", func(t *testing.T) {
		assert.Equal(t, "imported-name", srv.Name)
		assert.Equal(t, "server", srv.Type)
		assert.Equal(t, "imported-image", srv.Image)
		assert.Equal(t, "imported description", srv.Description)
		assert.Equal(t, "Imported Title", srv.Title)
		assert.Equal(t, "imported.png", srv.Icon)
		assert.Equal(t, "https://example.com/readme", srv.ReadmeURL)
		assert.Equal(t, "ok", srv.Prefix)
		require.NotNil(t, srv.Metadata)
		assert.Equal(t, "utility", srv.Metadata.Category)
		assert.Equal(t, []string{"community"}, srv.Metadata.Tags)
		assert.Len(t, srv.Config, 1)
	})

	t.Run("runtime-shaping fields dropped", func(t *testing.T) {
		assert.Empty(t, srv.Command, "Command must not be importable")
		assert.Empty(t, srv.Volumes, "Volumes must not be importable")
		assert.Empty(t, srv.User, "User must not be importable")
		assert.Empty(t, srv.ExtraHosts, "ExtraHosts must not be importable")
		assert.Empty(t, srv.AllowHosts, "AllowHosts must not be importable")
		assert.False(t, srv.DisableNetwork, "DisableNetwork must not be importable")
		assert.False(t, srv.LongLived, "LongLived must not be importable")
		assert.Empty(t, srv.SSEEndpoint, "SSEEndpoint must not be importable")
		assert.Equal(t, Remote{}, srv.Remote, "Remote must not be importable")
		assert.Nil(t, srv.OAuth, "OAuth must not be importable")
		assert.Nil(t, srv.Policy, "Policy must not be importable")
	})

	t.Run("env values dropped, names kept", func(t *testing.T) {
		require.Len(t, srv.Env, 2)
		assert.Equal(t, "LOG_LEVEL", srv.Env[0].Name)
		assert.Empty(t, srv.Env[0].Value)
		assert.Equal(t, "DOCKER_HOST", srv.Env[1].Name)
		assert.Empty(t, srv.Env[1].Value, "Env values must not be importable")
	})

	t.Run("secrets dropped from imports", func(t *testing.T) {
		assert.Empty(t, srv.Secrets, "Secrets must not be importable from OCI labels")
	})

	t.Run("tool container config dropped", func(t *testing.T) {
		require.Len(t, srv.Tools, 1)
		assert.Equal(t, "list_things", srv.Tools[0].Name)
		assert.Equal(t, "lists things", srv.Tools[0].Description)
		assert.Equal(t, Container{}, srv.Tools[0].Container, "Tool.Container must not be importable")
	})
}

func TestImportedServer_DropsRuntimeFields_JSON(t *testing.T) {
	// Same payload via JSON to exercise the JSON import path.
	payload := []byte(`{
		"name": "imported-json",
		"type": "server",
		"command": ["/bin/sh"],
		"volumes": ["/data:/data"],
		"user": "root",
		"env": [{"name": "DOCKER_HOST", "value": "tcp://other-host:2375"}]
	}`)

	var imported ImportedServer
	require.NoError(t, json.Unmarshal(payload, &imported))
	srv := imported.ToServer()

	assert.Equal(t, "imported-json", srv.Name)
	assert.Empty(t, srv.Command)
	assert.Empty(t, srv.Volumes)
	assert.Empty(t, srv.User)
	require.Len(t, srv.Env, 1)
	assert.Equal(t, "DOCKER_HOST", srv.Env[0].Name)
	assert.Empty(t, srv.Env[0].Value)
}
