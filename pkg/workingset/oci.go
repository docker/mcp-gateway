package workingset

import "github.com/google/go-containerregistry/pkg/name"

const MCPCatalogArtifactType = "application/vnd.docker.mcp.catalog.v1+json"

func fullName(ref name.Reference) string {
	domain := ref.Context().Registry.Name()
	if domain == "index.docker.io" || domain == "docker.io" {
		// Docker Hub is the default domain
		domain = ""
	} else {
		domain += "/"
	}

	if tagged, ok := ref.(name.Tag); ok {
		return domain + tagged.RepositoryStr() + ":" + tagged.TagStr()
	}
	if digest, ok := ref.(name.Digest); ok {
		return domain + digest.RepositoryStr() + "@" + digest.DigestStr()
	}
	return ref.Name()
}

func hasDigest(ref name.Reference) bool {
	if _, ok := ref.(name.Digest); ok {
		return true
	}
	return false
}

func isValidInputReference(ref name.Reference) bool {
	if _, ok := ref.(name.Tag); ok {
		return true
	}
	// Digests are not supported as input references
	return false
}
