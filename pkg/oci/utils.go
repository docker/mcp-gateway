package oci

import (
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
)

func FullName(ref name.Reference) string {
	domain := ref.Context().Registry.Name()
	if domain == "index.docker.io" || domain == "docker.io" {
		// Docker Hub is the default domain
		domain = ""
	} else {
		domain += "/"
	}

	// The parser will assume "library/" for non-namespaced repos, so we just trim that
	if tagged, ok := ref.(name.Tag); ok {
		return domain + strings.TrimPrefix(tagged.RepositoryStr(), "library/") + ":" + tagged.TagStr()
	}
	if digest, ok := ref.(name.Digest); ok {
		return domain + strings.TrimPrefix(digest.RepositoryStr(), "library/") + "@" + digest.DigestStr()
	}
	return ref.Name()
}

func FullNameWithoutDigest(ref name.Reference) string {
	domain := ref.Context().Registry.Name()
	if domain == "index.docker.io" || domain == "docker.io" {
		// Docker Hub is the default domain
		domain = ""
	} else {
		domain += "/"
	}

	// The parser will assume "library/" for non-namespaced repos, so we just trim that
	if tagged, ok := ref.(name.Tag); ok {
		return domain + strings.TrimPrefix(tagged.RepositoryStr(), "library/") + ":" + tagged.TagStr()
	}
	return domain + strings.TrimPrefix(ref.Context().RepositoryStr(), "library/") + ":latest"
}

func HasDigest(ref name.Reference) bool {
	if _, ok := ref.(name.Digest); ok {
		return true
	}
	return false
}

func IsValidInputReference(ref name.Reference) bool {
	if _, ok := ref.(name.Tag); ok {
		return true
	}
	// Digests are not supported as input references
	return false
}
