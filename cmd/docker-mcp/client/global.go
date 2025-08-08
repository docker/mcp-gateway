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
	Icon              string   `yaml:"icon"`
	InstallCheckPaths []string `yaml:"installCheckPaths"`
	Paths             `yaml:"paths"`
	YQ                `yaml:"yq"`
}

type Paths struct {
	Linux   []string `yaml:"linux"`
	Darwin  []string `yaml:"darwin"`
	Windows []string `yaml:"windows"`
}

func (c *globalCfg) GetPathsForCurrentOS() []string {
	switch runtime.GOOS {
	case "darwin":
		return c.Darwin
	case "linux":
		return c.Linux
	case "windows":
		return c.Windows
	}
	return []string{}
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
	paths := g.GetPathsForCurrentOS()
	if len(paths) == 0 {
		return nil, fmt.Errorf("no paths configured for OS %s", runtime.GOOS)
	}
	// All paths for a client must use same file format (json/yaml) since YQ processor
	// determines encoding from first path but may operate on any path
	p, err := newYQProcessor(g.YQ, paths[0])
	if err != nil {
		return nil, err
	}
	return &GlobalCfgProcessor{
		globalCfg: g,
		p:         *p,
	}, nil
}

func (c *GlobalCfgProcessor) ParseConfig() MCPClientCfg {
	result := MCPClientCfg{MCPClientCfgBase: MCPClientCfgBase{DisplayName: c.DisplayName, Source: c.Source, Icon: c.Icon}}

	paths := c.GetPathsForCurrentOS()
	if len(paths) == 0 {
		return result
	}
	result.IsOsSupported = true

	for _, path := range paths {
		fullPath := os.ExpandEnv(path)
		data, err := os.ReadFile(fullPath)
		if err == nil {
			result.IsInstalled = true
			result.setParseResult(c.p.Parse(data))
			return result
		}

		if os.IsNotExist(err) {
			continue
		}

		// File exists but can't be read. Because of an old bug, it could be a directory.
		// In which case, we want to delete it.
		stat, statErr := os.Stat(fullPath)
		if statErr == nil && stat.IsDir() {
			if rmErr := os.RemoveAll(fullPath); rmErr != nil {
				result.Err = classifyError(rmErr)
				return result
			}
			continue
		}

		result.IsInstalled = true
		result.Err = classifyError(err)
		return result
	}

	// No files found - check if the application is installed
	installed, installCheckErr := c.isInstalled()
	result.IsInstalled = installed
	result.Err = classifyError(installCheckErr)
	return result
}

func (c *GlobalCfgProcessor) Update(key string, server *MCPServerSTDIO) error {
	paths := c.GetPathsForCurrentOS()
	if len(paths) == 0 {
		return fmt.Errorf("unknown config path for OS %s", runtime.GOOS)
	}

	// Use first existing path, or first path if none exist
	var targetPath string
	for _, path := range paths {
		fullPath := os.ExpandEnv(path)
		if _, err := os.Stat(fullPath); err == nil {
			targetPath = fullPath
			break
		}
	}
	if targetPath == "" {
		targetPath = os.ExpandEnv(paths[0])
	}

	return updateConfig(targetPath, c.p.Add, c.p.Del, key, server)
}

func containsMCPDocker(in []MCPServerSTDIO) bool {
	for _, server := range in {
		if server.Name == DockerMCPCatalog || server.Name == makeSimpleName(DockerMCPCatalog) {
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
