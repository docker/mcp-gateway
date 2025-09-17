package gateway

import (
	"context"
	"os"
	"sort"
)

func (g *Gateway) guessNetworks(ctx context.Context) ([]string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	found, response, err := g.docker.ContainerExists(ctx, hostname)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}

	var networks []string
	for network := range response.NetworkSettings.Networks {
		networks = append(networks, network)
	}
	sort.Strings(networks)

	return networks, nil
}
