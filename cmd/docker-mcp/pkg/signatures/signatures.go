package signatures

import (
	"context"
	"crypto"
	"errors"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	"github.com/sigstore/cosign/v2/pkg/oci"
	ociremote "github.com/sigstore/cosign/v2/pkg/oci/remote"
	"github.com/sigstore/cosign/v2/pkg/oci/static"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"
	"golang.org/x/sync/errgroup"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/version"
)

// https://raw.githubusercontent.com/docker/keyring/refs/heads/main/public/mcp/latest.pub
const publicKey = `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAETcvGv5qypnoxKkFIQOdXZ+2H6JN8
8kmAQrMkTb6SmJ7BY59OJIOpTwdjD5joLot6zFs1Q7HHDmkF5HOaC8zSnA==
-----END PUBLIC KEY-----`

func Verify(ctx context.Context, images []string) error {
	pubKey, err := cryptoutils.UnmarshalPEMToPublicKey([]byte(publicKey))
	if err != nil {
		return fmt.Errorf("pem to public key: %w", err)
	}

	sigVerifier, err := signature.LoadVerifier(pubKey, crypto.SHA256)
	if err != nil {
		return fmt.Errorf("loading public key: %w", err)
	}

	signatures, err := name.NewRepository("mcp/signatures")
	if err != nil {
		return err
	}

	rekor, err := cosign.GetRekorPubs(ctx)
	if err != nil {
		return fmt.Errorf("getting Rekor public keys: %w", err)
	}

	errs, ctxVerify := errgroup.WithContext(ctx)
	errs.SetLimit(2)
	for _, img := range images {
		errs.Go(func() error {
			ref, err := name.NewDigest(img)
			if err != nil {
				return fmt.Errorf("parsing reference: %w", err)
			}

			bundleVerified, err := verifyImageSignatures(ctx, ref, &cosign.CheckOpts{
				RegistryClientOpts: []ociremote.Option{
					ociremote.WithTargetRepository(signatures),
					ociremote.WithRemoteOptions(
						remote.WithContext(ctxVerify),
						remote.WithUserAgent(version.UserAgent()),
						// remote.WithAuthFromKeychain(authn.DefaultKeychain),
					),
				},
				RekorPubKeys: rekor,
				SigVerifier:  sigVerifier,
			})
			if err != nil {
				return err
			}

			if !bundleVerified {
				return errors.New("bundle verification failed")
			}

			return nil
		})
	}

	return errs.Wait()
}

// verifyImageSignatures is copied from cosign in order to not depend on a bunch of transitivie dependencies.
func verifyImageSignatures(ctx context.Context, digest name.Digest, co *cosign.CheckOpts) (bundleVerified bool, err error) {
	h, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return false, err
	}

	var sigs oci.Signatures
	st, err := ociremote.SignatureTag(digest, co.RegistryClientOpts...)
	if err != nil {
		return false, err
	}
	sigs, err = ociremote.Signatures(st, co.RegistryClientOpts...)
	if err != nil {
		return false, err
	}
	sl, err := sigs.Get()
	if err != nil {
		return false, err
	}
	if len(sl) == 0 {
		return false, errors.New("no signatures found")
	}

	for _, sig := range sl {
		sig, err := static.Copy(sig)
		if err != nil {
			return false, err
		}

		verified, err := cosign.VerifyImageSignature(ctx, sig, h, co)
		if err != nil {
			return false, err
		}

		if !verified {
			return false, nil
		}
	}

	return true, nil
}
