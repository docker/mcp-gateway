package gateway

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/distribution/reference"

	"github.com/docker/mcp-gateway/pkg/log"
	"github.com/docker/mcp-gateway/pkg/signatures"
)

var verifyDockerImageSignatures = signatures.Verify

func (g *Gateway) pullAndVerify(ctx context.Context, configuration Configuration) error {
	dockerImages := configuration.DockerImages()
	if len(dockerImages) == 0 {
		return nil
	}

	log.Log("- Using images:")

	var verifiableImages []string
	for _, image := range dockerImages {
		log.Log("  - " + image)
		if isDockerMCPImage(image) {
			verifiableImages = append(verifiableImages, image)
		}
	}

	if err := g.verifyImages(ctx, verifiableImages); err != nil {
		return err
	}

	if err := g.pullImages(ctx, dockerImages); err != nil {
		return err
	}

	return nil
}

func (g *Gateway) pullAndVerifyImage(ctx context.Context, image string) error {
	if image == "" {
		return nil
	}

	if isDockerMCPImage(image) {
		if err := g.verifyImages(ctx, []string{image}); err != nil {
			return err
		}
	}

	return g.pullImages(ctx, []string{image})
}

func (g *Gateway) pullImages(ctx context.Context, images []string) error {
	start := time.Now()

	if err := g.docker.PullImages(ctx, images...); err != nil {
		return fmt.Errorf("pulling docker images: %w", err)
	}

	log.Log("> Images pulled in", time.Since(start))
	return nil
}

func (g *Gateway) verifyImages(ctx context.Context, images []string) error {
	if len(images) == 0 {
		return nil
	}

	if !g.VerifySignatures {
		log.Log("Warning: signature verification is disabled; MCP server images will not be verified and may use mutable tags")
		return nil
	}

	for _, image := range images {
		if !strings.Contains(image, "@sha256:") {
			return fmt.Errorf("verifying docker image %s: image must be referenced by digest; pin the MCP image to a sha256 digest or disable signature verification with --verify-signatures=false", image)
		}
	}

	start := time.Now()
	log.Log("- Verifying images", imageBaseNames(images))

	if err := verifyDockerImageSignatures(ctx, images); err != nil {
		return fmt.Errorf("verifying docker images: %w", err)
	}

	log.Log("> Images verified in", time.Since(start))
	return nil
}

func isDockerMCPImage(image string) bool {
	named, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return false
	}

	switch reference.Domain(named) {
	case "docker.io", "registry-1.docker.io":
		return strings.HasPrefix(reference.Path(named), "mcp/")
	default:
		return false
	}
}

func imageBaseNames(names []string) []string {
	baseNames := make([]string, len(names))

	for i, name := range names {
		baseNames[i] = imageBaseName(name)
	}

	return baseNames
}

func imageBaseName(name string) string {
	before, _, found := strings.Cut(name, "@sha256:")
	if found {
		return before
	}

	return name
}
