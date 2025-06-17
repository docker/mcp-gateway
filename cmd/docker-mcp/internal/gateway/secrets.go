package gateway

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/desktop"
)

const jcatImage = "docker/jcat@sha256:76719466e8b99a65dd1d37d9ab94108851f009f0f687dce7ff8a6fc90575c4d4"

type Secret struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func secretValues(ctx context.Context, names []string) (map[string]string, error) {
	flags := []string{"--network=none", "--pull=missing"}
	var command []string

	for i, name := range names {
		file := fmt.Sprintf("/.s%d", i)
		flags = append(flags, "-l", "x-secret:"+name+"="+file)
		command = append(command, file)
	}

	var args []string
	args = append(args, flags...)
	args = append(args, jcatImage)
	args = append(args, command...)

	buf, err := desktop.RunWithRawDockerSocket(ctx, args...)
	if err != nil {
		return nil, err
	}

	var list []string
	if err := json.Unmarshal(buf, &list); err != nil {
		return nil, err
	}

	values := map[string]string{}
	for i := range names {
		values[names[i]] = list[i]
	}

	return values, nil
}
