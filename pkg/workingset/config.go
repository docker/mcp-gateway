package workingset

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/mcp-gateway/pkg/user"
	"gopkg.in/yaml.v3"
)

type WorkingSetIndex struct {
	WorkingSets map[string]struct {
		Path string `yaml:"path" json:"path"`
	} `yaml:"workingSets" json:"workingSets"`
}

func Write(workingSet WorkingSet) error {
	index, err := readIndexOrEmpty()
	if err != nil {
		return err
	}

	path, err := filePath(workingSet.ID + ".yaml")
	if err != nil {
		return err
	}

	index.WorkingSets[workingSet.ID] = struct {
		Path string `yaml:"path" json:"path"`
	}{Path: path}

	data, err := yaml.Marshal(workingSet)
	if err != nil {
		return err
	}

	if err := writeWorkingSetFile(path, data); err != nil {
		return err
	}

	if err := writeIndex(index); err != nil {
		return err
	}

	return nil
}

func Read(id string) (*WorkingSet, error) {
	index, err := readIndexOrEmpty()
	if err != nil {
		return nil, err
	}

	if _, found := index.WorkingSets[id]; !found {
		return nil, fmt.Errorf("working set %s not found", id)
	}

	workingSetBuf, err := readWorkingSetFile(index.WorkingSets[id].Path)
	if err != nil {
		return nil, err
	}

	var workingSet WorkingSet
	if err := yaml.Unmarshal(workingSetBuf, &workingSet); err != nil {
		return nil, err
	}

	return &workingSet, nil
}

func writeIndex(index *WorkingSetIndex) error {
	path, err := filePath("index.json")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

func readIndexOrEmpty() (*WorkingSetIndex, error) {
	buf, err := readWorkingSetFile("index.json")
	if err != nil {
		if os.IsNotExist(err) {
			return &WorkingSetIndex{
				WorkingSets: map[string]struct {
					Path string `yaml:"path" json:"path"`
				}{},
			}, nil
		}
		return nil, err
	}

	var index WorkingSetIndex
	if err := json.Unmarshal(buf, &index); err != nil {
		return nil, err
	}
	return &index, nil
}

func writeWorkingSetFile(filename string, data []byte) error {
	path, err := filePath(filename)
	if err != nil {
		return err
	}

	_ = os.MkdirAll(filepath.Dir(path), 0o755)

	return os.WriteFile(path, data, 0o644)
}

func readWorkingSetFile(filename string) ([]byte, error) {
	path, err := filePath(filename)
	if err != nil {
		return nil, err
	}
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func filePath(filename string) (string, error) {
	if filepath.IsAbs(filename) {
		return filename, nil
	}
	if strings.HasPrefix(filename, "./") {
		return filepath.Abs(filename)
	}
	homeDir, err := user.HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".docker", "mcp", "workingset", filename), nil
}
