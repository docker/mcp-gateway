package gateway

import (
	"fmt"
	"math/big"
	"slices"
	"strconv"
	"strings"

	"github.com/docker/go-units"

	"github.com/docker/mcp-gateway/pkg/catalog"
)

func globalCPUs(cpus int) string {
	if cpus <= 0 {
		return ""
	}
	return strconv.Itoa(cpus)
}

// ValidateResourceOverrides checks operator-supplied limits before any I/O.
// Names need not be active at startup: overrides also apply to dynamic servers.
func (o Options) ValidateResourceOverrides() error {
	for _, field := range []struct {
		flag   string
		values map[string]string
		parse  func(string) (int64, error)
	}{{"server-cpus", o.ServerCPUs, parseResourceCPUs}, {"server-memory", o.ServerMemory, parseResourceMemory}} {
		names := make([]string, 0, len(field.values))
		for name := range field.values {
			names = append(names, name)
		}
		slices.Sort(names)
		for _, name := range names {
			if name == "" || strings.TrimSpace(name) != name {
				return fmt.Errorf("--%s: server name must be non-empty and have no surrounding whitespace", field.flag)
			}
			if _, err := field.parse(field.values[name]); err != nil {
				return fmt.Errorf("--%s for server %q: %w", field.flag, name, err)
			}
		}
	}
	return nil
}

func (o Options) validateServerResources(server *catalog.ServerConfig) error {
	// Remote and static servers are not containers launched by this gateway.
	if o.Static || server.IsRemote() || server.Spec.Image == "" {
		return nil
	}
	_, err := o.serverResources(server)
	return err
}

func (o Options) serverResources(server *catalog.ServerConfig) (catalog.Resources, error) {
	requested := catalog.Resources{}
	if server.Spec.Resources != nil {
		requested = *server.Spec.Resources
	}
	cpus, err := resolveResource(server.Name, "cpus", requested.CPUs, globalCPUs(o.Cpus), o.ServerCPUs, parseResourceCPUs)
	if err != nil {
		return catalog.Resources{}, err
	}
	memory, err := resolveResource(server.Name, "memory", requested.Memory, o.Memory, o.ServerMemory, parseResourceMemory)
	if err != nil {
		return catalog.Resources{}, err
	}
	return catalog.Resources{CPUs: cpus, Memory: memory}, nil
}

// An operator override is authoritative for that field. Catalog limits can only
// reduce a bounded global default. Reject, rather than silently clamp, requests
// above that default so the operator knows the workload may need more resources.
func resolveResource(name, field, requested, fallback string, overrides map[string]string, parse func(string) (int64, error)) (string, error) {
	if value, ok := overrides[name]; ok {
		if _, err := parse(value); err != nil {
			return "", fmt.Errorf("server %q --server-%s: %w", name, field, err)
		}
		return value, nil
	}
	if requested == "" {
		return fallback, nil
	}
	value, err := parse(requested)
	if err != nil {
		return "", fmt.Errorf("server %q resources.%s: %w", name, field, err)
	}
	if fallback != "" {
		// Docker accepts a zero global memory limit as unlimited. Preserve that
		// opt-out while still requiring positive per-server limits.
		if field == "memory" {
			if bytes, err := units.RAMInBytes(fallback); err == nil && bytes == 0 {
				return requested, nil
			}
		}
		limit, err := parse(fallback)
		if err != nil {
			return "", fmt.Errorf("server %q: invalid global --%s: %w", name, field, err)
		}
		if value > limit {
			return "", fmt.Errorf("server %q resources.%s=%q exceeds global --%s=%q; explicitly authorize it with --server-%s %s=%s", name, field, requested, field, fallback, field, name, requested)
		}
	}
	return requested, nil
}

func parseResourceCPUs(value string) (int64, error) {
	// Use exact arithmetic: Docker represents CPUs in nanocpus. Reject overflow
	// and excess precision instead of rounding a limit or turning it into zero.
	if value == "" || strings.Trim(value, "0123456789.") != "" {
		return 0, fmt.Errorf("CPU limit %q must be a positive decimal number", value)
	}
	cpus, ok := new(big.Rat).SetString(value)
	if !ok {
		return 0, fmt.Errorf("invalid CPU limit %q", value)
	}
	nano := cpus.Mul(cpus, big.NewRat(1e9, 1))
	if !nano.IsInt() || !nano.Num().IsInt64() || nano.Num().Int64() < 10_000_000 {
		return 0, fmt.Errorf("CPU limit %q must be at least 0.01, fit in int64 nanocpus, and have at most 9 decimal places", value)
	}
	return nano.Num().Int64(), nil
}

func parseResourceMemory(value string) (int64, error) {
	size, err := units.RAMInBytes(value)
	if err != nil || size < 6*1024*1024 {
		return 0, fmt.Errorf("memory limit %q must be a valid Docker size of at least 6MiB", value)
	}
	return size, nil
}
