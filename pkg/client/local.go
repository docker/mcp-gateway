package client

import (
	"os"
	"path/filepath"
)

type localCfg struct {
	DisplayName string
	ProjectFile string
	Icon        string `yaml:"icon"`
	YQ          `yaml:"yq"`
}

type LocalCfgProcessor struct {
	localCfg
	p           yqProcessor
	projectRoot string
}

func NewLocalCfgProcessor(cfg localCfg, projectRoot string) (*LocalCfgProcessor, error) {
	p, err := newYQProcessor(cfg.YQ, cfg.ProjectFile)
	if err != nil {
		return nil, err
	}
	return &LocalCfgProcessor{
		localCfg:    cfg,
		p:           *p,
		projectRoot: projectRoot,
	}, nil
}

func (c *LocalCfgProcessor) Parse() ProjectMCPClientCfg {
	result := ProjectMCPClientCfg{MCPClientCfgBase: MCPClientCfgBase{DisplayName: c.DisplayName, Icon: c.Icon}}
	file := filepath.Join(c.projectRoot, c.ProjectFile)
	dir := filepath.Dir(file)
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) { // it's not an error for us, it just means nothing is configured/connected
			return result
		}
		result.Err = classifyError(err)
		return result
	}
	result.IsConfigured = true
	data, err := os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) { // it's not an error for us, it just means nothing is configured/connected
			return result
		}
		result.Err = classifyError(err)
		return result
	}
	result.setParseResult(c.p.Parse(data))
	return result
}

func (c *LocalCfgProcessor) Update(key string, server *MCPServerSTDIO) error {
	return updateConfig(filepath.Join(c.projectRoot, c.ProjectFile), c.p.Add, c.p.Del, key, server)
}

type ProjectMCPClientCfg struct {
	MCPClientCfgBase
	IsConfigured bool `json:"isConfigured"`
}
