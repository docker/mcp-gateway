package client

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const (
	DockerMCPCatalog = "MCP_DOCKER"
)

type globalCfg struct {
	DisplayName       string   `yaml:"displayName"`
	Source            string   `yaml:"source"`
	InstallCheckPaths []string `yaml:"installCheckPaths"`
	Paths             `yaml:"paths"`
	YQ                `yaml:"yq"`
}

type Paths struct {
	Linux   string `yaml:"linux"`
	Darwin  string `yaml:"darwin"`
	Windows string `yaml:"windows"`
}

func (c *globalCfg) GetPathsForCurrentOS() string {
	switch runtime.GOOS {
	case "darwin":
		return c.Darwin
	case "linux":
		return c.Linux
	case "windows":
		return c.Windows
	}
	return ""
}

func (c *globalCfg) isInstalled() (bool, error) {
	var lastErr error
	for _, path := range c.InstallCheckPaths {
		_, err := os.Stat(os.ExpandEnv(path))
		if err == nil {
			return true, nil
		}
		if !os.IsNotExist(err) {
			lastErr = err
		}
	}
	return false, lastErr
}

type GlobalCfgProcessor struct {
	globalCfg
	p yqProcessor
}

func NewGlobalCfgProcessor(g globalCfg) (*GlobalCfgProcessor, error) {
	p, err := newYQProcessor(g.YQ, g.GetPathsForCurrentOS())
	if err != nil {
		return nil, err
	}
	return &GlobalCfgProcessor{
		globalCfg: g,
		p:         *p,
	}, nil
}

func (c *GlobalCfgProcessor) ParseConfig() MCPClientCfg {
	result := MCPClientCfg{MCPClientCfgBase: MCPClientCfgBase{DisplayName: c.DisplayName, Source: c.Source}}

	path := c.GetPathsForCurrentOS()
	if path == "" {
		return result
	}
	fullPath := os.ExpandEnv(path)
	result.IsOsSupported = true

	data, err := os.ReadFile(fullPath)
	if os.IsNotExist(err) {
		// it's not an error for us, it just means nothing is configured/connected
		installed, installCheckErr := c.isInstalled()
		result.IsInstalled = installed
		result.Err = classifyError(installCheckErr)
		return result
	}

	// The file was found but can't be read. Because of an old bug, it could be a directory.
	// In which case, we want to delete it.
	stat, err := os.Stat(fullPath)
	if err == nil && stat.IsDir() {
		if err := os.RemoveAll(fullPath); err != nil {
			result.Err = classifyError(err)
			return result
		}
		installed, installCheckErr := c.isInstalled()
		result.IsInstalled = installed
		result.Err = classifyError(installCheckErr)
		return result
	}

	// config exists for us means it's installed (we then don't care if it's actually installed or not)
	result.IsInstalled = true
	if err != nil {
		result.Err = classifyError(err)
		return result
	}
	result.setParseResult(c.p.Parse(data))
	return result
}

func (c *GlobalCfgProcessor) Update(key string, server *MCPServerSTDIO) error {
	file := c.GetPathsForCurrentOS()
	if file == "" {
		return fmt.Errorf("unknown config path for OS %s", runtime.GOOS)
	}
	return updateConfig(os.ExpandEnv(file), c.p.Add, c.p.Del, key, server)
}

func containsMCPDocker(in []MCPServerSTDIO) bool {
	for _, server := range in {
		if server.Name == DockerMCPCatalog {
			return true
		}
	}
	return false
}

type (
	cfgAdd func([]byte, MCPServerSTDIO) ([]byte, error)
	cfgDel func([]byte, string) ([]byte, error)
)

func updateConfig(file string, add cfgAdd, del cfgDel, key string, server *MCPServerSTDIO) error {
	dir := filepath.Dir(file)
	if _, err := os.Stat(dir); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(file)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	data, err = del(data, key)
	if err != nil {
		return err
	}
	if server != nil {
		server.Name = key
		data, err = add(data, *server)
		if err != nil {
			return err
		}
	}

	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	return os.WriteFile(file, data, 0o644)
}

type MCPClientCfg struct {
	MCPClientCfgBase
	IsInstalled   bool `json:"isInstalled"`
	IsOsSupported bool `json:"isOsSupported"`
}

func classifyError(err error) *CfgError {
	if err == nil {
		return nil
	}
	errType := "unknown"
	if os.IsPermission(err) {
		errType = "permission"
	}
	return &CfgError{
		Type: errType,
		Err:  err.Error(),
	}
}

type CfgError struct {
	Type string `json:"type"` // permission|unknown
	Err  string `json:"error"`
}
