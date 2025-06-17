package client

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/mikefarah/yq/v4/pkg/yqlib"

	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/yq"
)

type YQ struct {
	List string `yaml:"list"`
	Set  string `yaml:"set"`
	Del  string `yaml:"del"`
}

type yqProcessor struct {
	YQ
	decoder yqlib.Decoder
	encoder yqlib.Encoder
}

func newYQProcessor(yq YQ, path string) (*yqProcessor, error) {
	decoder, encoder, err := inferEncoding(path)
	if err != nil {
		return nil, err
	}
	return &yqProcessor{
		YQ:      yq,
		decoder: decoder,
		encoder: encoder,
	}, nil
}

func (c *yqProcessor) Parse(data []byte) (*MCPJSONLists, error) {
	tmpJSON, err := yq.Evaluate(c.List, data, c.decoder, yq.NewJSONEncoder())
	if err != nil {
		return nil, err
	}
	return UnmarshalMCPJSONList(tmpJSON)
}

func inferEncoding(path string) (yqlib.Decoder, yqlib.Encoder, error) {
	switch filepath.Ext(path) {
	case ".json":
		return yqlib.NewJSONDecoder(), yq.NewJSONEncoder(), nil
	case ".yaml", ".yml":
		return yq.NewYamlDecoder(), yq.NewYamlEncoder(), nil
	default:
		return nil, nil, errors.New("unsupported file type")
	}
}

func (c *yqProcessor) Del(data []byte, key string) ([]byte, error) {
	return yq.Evaluate(os.Expand(c.YQ.Del, func(s string) string { return expandDelQuery(s, key) }), data, c.decoder, c.encoder)
}

func expandDelQuery(name string, key string) string {
	switch name {
	case "NAME":
		return stringEscape(key)
	default:
		return ""
	}
}

func (c *yqProcessor) Add(data []byte, server MCPServerSTDIO) ([]byte, error) {
	if len(data) == 0 {
		data = []byte("null")
	}
	expression := os.Expand(c.Set, func(s string) string { return expandSetQuery(s, server) })
	return yq.Evaluate(expression, data, c.decoder, c.encoder)
}

func stringEscape(s string) string {
	return `"` + s + `"`
}

func expandSetQuery(name string, server MCPServerSTDIO) string {
	temp := struct {
		Command string            `json:"command"`
		Args    []string          `json:"args,omitempty"`
		Env     map[string]string `json:"env,omitempty"`
	}{
		Command: server.Command,
		Args:    server.Args,
		Env:     server.Env,
	}
	switch name {
	case "NAME":
		return stringEscape(server.Name)
	case "JSON":
		result, err := json.Marshal(temp)
		if err != nil {
			return ""
		}
		return string(result)
	default:
		return ""
	}
}
