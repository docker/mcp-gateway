package catalognext

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/oci"
	"github.com/docker/mcp-gateway/pkg/policy"
	"github.com/docker/mcp-gateway/pkg/validate"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

type CatalogArtifact struct {
	Title   string   `yaml:"title" json:"title" validate:"required,min=1"`
	Servers []Server `yaml:"servers" json:"servers" validate:"dive"`
}

type Catalog struct {
	Ref             string `yaml:"ref" json:"ref" validate:"required,min=1"`
	Source          string `yaml:"source,omitempty" json:"source,omitempty"`
	CatalogArtifact `yaml:",inline"`
	// Policy describes the policy decision for this catalog.
	Policy *policy.Decision `yaml:"policy,omitempty" json:"policy,omitempty"`
}

type CatalogWithDigest struct {
	Catalog `yaml:",inline"`
	Digest  string `yaml:"digest" json:"digest"`
}

type CatalogSummary struct {
	Ref    string `yaml:"ref" json:"ref"`
	Digest string `yaml:"digest" json:"digest"`
	Title  string `yaml:"title" json:"title"`
	// Policy describes the policy decision for this catalog summary.
	Policy *policy.Decision `yaml:"policy,omitempty" json:"policy,omitempty"`
}

// Source prefixes must be of the form "<prefix>:"
const (
	SourcePrefixWorkingSet    = "profile:"
	SourcePrefixLegacyCatalog = "legacy-catalog:"
	SourcePrefixOCI           = "oci:"
	SourcePrefixUser          = "user:"
	SourcePrefixRegistry      = "registry:"
)

type Server struct {
	Type  workingset.ServerType `yaml:"type" json:"type" validate:"required,oneof=registry image remote"`
	Tools []string              `yaml:"tools,omitempty" json:"tools,omitempty"`
	// Policy describes the policy decision for this server.
	Policy *policy.Decision `yaml:"policy,omitempty" json:"policy,omitempty"`

	// ServerTypeRegistry only
	Source string `yaml:"source,omitempty" json:"source,omitempty" validate:"required_if=Type registry"`

	// ServerTypeImage only
	Image string `yaml:"image,omitempty" json:"image,omitempty" validate:"required_if=Type image"`

	// ServerTypeRemote only
	Endpoint string `yaml:"endpoint,omitempty" json:"endpoint,omitempty" validate:"required_if=Type remote"`

	Snapshot *workingset.ServerSnapshot `yaml:"snapshot,omitempty" json:"snapshot,omitempty"`
}

func NewFromDb(dbCatalog *db.Catalog) CatalogWithDigest {
	servers := make([]Server, len(dbCatalog.Servers))
	for i, server := range dbCatalog.Servers {
		servers[i] = Server{
			Type:  workingset.ServerType(server.ServerType),
			Tools: server.Tools,
		}
		if server.ServerType == "registry" {
			servers[i].Source = server.Source
		}
		if server.ServerType == "image" {
			servers[i].Image = server.Image
		}
		if server.ServerType == "remote" {
			servers[i].Endpoint = server.Endpoint
		}
		if server.Snapshot != nil {
			servers[i].Snapshot = &workingset.ServerSnapshot{
				Server: server.Snapshot.Server,
			}
		}
	}

	catalog := CatalogWithDigest{
		Catalog: Catalog{
			Ref:    dbCatalog.Ref,
			Source: dbCatalog.Source,
			CatalogArtifact: CatalogArtifact{
				Title:   dbCatalog.Title,
				Servers: servers,
			},
		},
		Digest: dbCatalog.Digest,
	}

	return catalog
}

func (catalog Catalog) ToDb() (db.Catalog, error) {
	dbServers := make([]db.CatalogServer, len(catalog.Servers))
	for i, server := range catalog.Servers {
		dbServers[i] = db.CatalogServer{
			ServerType: string(server.Type),
			Tools:      server.Tools,
		}
		if server.Type == workingset.ServerTypeRegistry {
			dbServers[i].Source = server.Source
		}
		if server.Type == workingset.ServerTypeImage {
			dbServers[i].Image = server.Image
		}
		if server.Type == workingset.ServerTypeRemote {
			dbServers[i].Endpoint = server.Endpoint
		}
		if server.Snapshot != nil {
			dbServers[i].Snapshot = &db.ServerSnapshot{
				Server: server.Snapshot.Server,
			}
		}
	}

	digest, err := catalog.Digest()
	if err != nil {
		return db.Catalog{}, fmt.Errorf("failed to get catalog digest: %w", err)
	}

	return db.Catalog{
		Ref:     catalog.Ref,
		Digest:  digest,
		Title:   catalog.Title,
		Source:  catalog.Source,
		Servers: dbServers,
	}, nil
}

func (catalogArtifact *CatalogArtifact) Digest() (string, error) {
	return oci.GetArtifactDigest(MCPCatalogArtifactType, catalogArtifact)
}

func (catalog *Catalog) FindServer(serverName string) *Server {
	for i := range len(catalog.Servers) {
		if catalog.Servers[i].Snapshot == nil {
			// TODO(cody): Can happen with registry (for now)
			continue
		}
		if catalog.Servers[i].Snapshot.Server.Name == serverName {
			return &catalog.Servers[i]
		}
	}
	return nil
}

func (catalog *Catalog) Validate() error {
	if err := validate.Get().Struct(catalog); err != nil {
		return err
	}
	if err := catalog.validateUniqueServerNames(); err != nil {
		return err
	}
	return catalog.validateServerSnapshots()
}

func (catalog *Catalog) validateServerSnapshots() error {
	for _, server := range catalog.Servers {
		if err := server.Snapshot.ValidateInnerConfig(); err != nil {
			return err
		}
	}
	return nil
}

func (catalog *Catalog) validateUniqueServerNames() error {
	seen := make(map[string]bool)
	for _, server := range catalog.Servers {
		// TODO: Update when Snapshot is required
		if server.Snapshot == nil {
			continue
		}
		name := server.Snapshot.Server.Name
		if seen[name] {
			return fmt.Errorf("duplicate server name %s", name)
		}
		seen[name] = true
	}
	return nil
}

type PullOption string

// Options can be used in combination with each other, e.g. "initial+exists@6h".
const (
	PullOptionMissing = "missing"
	PullOptionNever   = "never"
	PullOptionAlways  = "always"
	PullOptionInitial = "initial"
	PullOptionExists  = "exists"

	// Special value for duration-based pull options. Don't add as supported pull option below.
	// Can be used in combination with exists, initial, or always by appending @duration (e.g. "exists@1h", "always@1w").
	PullOptionDuration = "duration"
)

func SupportedPullOptions() []string {
	return []string{string(PullOptionMissing), string(PullOptionNever), string(PullOptionAlways), string(PullOptionInitial), string(PullOptionExists)}
}

type PullOptionConfig struct {
	PullOption PullOption
	Interval   time.Duration
}

type PullOptionEvaluator struct {
	pullOptions      []PullOptionConfig
	pulledPreviously bool
}

func NewPullOptionEvaluator(pullOptionParam string, pulledPreviously bool) (*PullOptionEvaluator, error) {
	pullOptions := []PullOptionConfig{}
	parts := strings.Split(pullOptionParam, "+")
	if len(parts) == 0 || pullOptionParam == "" {
		return &PullOptionEvaluator{pullOptions: pullOptions, pulledPreviously: pulledPreviously}, nil
	}

	for _, part := range parts {
		var pullOption PullOption
		var pullInterval time.Duration

		innerParts := strings.Split(part, "@")
		if len(innerParts) == 0 {
			continue
		}
		if len(innerParts) == 2 { // e.g. 'always@6h'
			duration, err := parseDuration(innerParts[1])
			if err != nil {
				return nil, err
			}
			pullInterval = duration
		}
		// else assume just e.g. 'always' or '6h'

		isPullOption := slices.Contains(SupportedPullOptions(), innerParts[0])
		if isPullOption {
			pullOption = PullOption(innerParts[0])
		} else if pullInterval > 0 { // Already set with an @
			return nil, fmt.Errorf("invalid pull option %s: should be %s", innerParts[0], strings.Join(SupportedPullOptions(), ", "))
		} else {
			// Must be a duration, e.g. '6h'
			duration, err := parseDuration(innerParts[0])
			if err != nil {
				return nil, err
			}
			pullInterval = duration
			pullOption = PullOptionDuration
		}
		pullOptions = append(pullOptions, PullOptionConfig{
			PullOption: pullOption,
			Interval:   pullInterval,
		})
	}

	return &PullOptionEvaluator{pullOptions: pullOptions, pulledPreviously: pulledPreviously}, nil
}

func parseDuration(durationStr string) (time.Duration, error) {
	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse pull interval duration, should be a duration format (e.g. '1h', '1d'). You gave %s: %w", durationStr, err)
	}
	if duration < 0 {
		return 0, fmt.Errorf("duration %s must be positive", duration)
	}
	return duration, nil
}

func (evaluator *PullOptionEvaluator) IsAlways() bool {
	for _, pullOption := range evaluator.pullOptions {
		if pullOption.PullOption == PullOptionAlways && pullOption.Interval == 0 {
			// Always and at no interval will be always unconditionally
			return true
		}
	}
	return false
}

func (evaluator *PullOptionEvaluator) Evaluate(dbCatalog *db.Catalog) bool {
	for _, pullOption := range evaluator.pullOptions {
		// If any pull option is true, return true
		if pullOption.Evaluate(dbCatalog, evaluator.pulledPreviously) {
			return true
		}
	}
	return false
}

func (pullOptionConfig PullOptionConfig) Evaluate(dbCatalog *db.Catalog, pulledPreviously bool) bool {
	if dbCatalog == nil {
		switch pullOptionConfig.PullOption {
		case PullOptionMissing, PullOptionAlways, PullOptionDuration:
			return true // Always pull immediately when not found, even if there's a duration
		case PullOptionInitial:
			return !pulledPreviously
		default:
			return false
		}
	}

	switch pullOptionConfig.PullOption {
	case PullOptionMissing, PullOptionNever, PullOptionInitial:
		return false
	case PullOptionDuration, PullOptionExists, PullOptionAlways:
		return pullOptionConfig.intervalPassed(dbCatalog)
	default:
		return false
	}
}

func (pullOptionConfig PullOptionConfig) intervalPassed(dbCatalog *db.Catalog) bool {
	// Special case: no interval means always past
	if pullOptionConfig.Interval == 0 {
		return true
	}

	// Only if last updated was longer ago than the interval
	if dbCatalog != nil && dbCatalog.LastUpdated != nil && time.Since(*dbCatalog.LastUpdated) > pullOptionConfig.Interval {
		return true
	}

	return false
}
