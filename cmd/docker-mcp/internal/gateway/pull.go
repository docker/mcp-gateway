package gateway

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/signatures"
)

func (g *Gateway) pullAndVerify(ctx context.Context, configuration Configuration) error {
	dockerImages := configuration.DockerImages()
	if len(dockerImages) == 0 {
		return nil
	}

	log("- Using images:")

	var verifiableImages []string
	for _, image := range dockerImages {
		log("  - " + image)
		if strings.HasPrefix(image, "mcp/") {
			verifiableImages = append(verifiableImages, image)
		}
	}

	if err := g.pullImages(ctx, dockerImages); err != nil {
		return err
	}

	if err := g.verifyImages(ctx, verifiableImages); err != nil {
		return err
	}

	return nil
}

func (g *Gateway) pullImages(ctx context.Context, images []string) error {
	start := time.Now()

	if err := g.docker.PullImages(ctx, images...); err != nil {
		return fmt.Errorf("pulling docker images: %w", err)
	}

	log("> Images pulled in", time.Since(start))
	return nil
}

func (g *Gateway) verifyImages(ctx context.Context, images []string) error {
	if !g.VerifySignatures {
		return nil
	}

	start := time.Now()
	log("- Verifying images", imageBaseNames(images))

	if err := signatures.Verify(ctx, images); err != nil {
		return fmt.Errorf("verifying docker images: %w", err)
	}

	log("> Images verified in", time.Since(start))
	return nil
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
