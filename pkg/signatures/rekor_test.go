package signatures

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sigstore/cosign/v2/pkg/cosign"
	"github.com/stretchr/testify/require"
)

func TestIsTUFCachedStoreLockError(t *testing.T) {
	winErr := errors.New(`creating cached local store: The process cannot access the file because it is being used by another process.`)
	require.True(t, isTUFCachedStoreLockError(winErr))

	unixErr := errors.New("creating cached local store: resource temporarily unavailable")
	require.True(t, isTUFCachedStoreLockError(unixErr))

	otherCacheErr := errors.New("creating cached local store: permission denied")
	require.False(t, isTUFCachedStoreLockError(otherCacheErr))

	require.False(t, isTUFCachedStoreLockError(errors.New("getting Rekor public keys: network timeout")))
}

func TestGetRekorPublicKeysWithRetry_SucceedsAfterCacheLock(t *testing.T) {
	old := getRekorPublicKeys
	defer func() { getRekorPublicKeys = old }()

	lockErr := errors.New("creating cached local store: The process cannot access the file because it is being used by another process.")
	calls := 0
	getRekorPublicKeys = func(_ context.Context) (*cosign.TrustedTransparencyLogPubKeys, error) {
		calls++
		if calls == 1 {
			return nil, lockErr
		}
		keys := cosign.NewTrustedTransparencyLogPubKeys()
		return &keys, nil
	}

	keys, err := getRekorPublicKeysWithRetry(context.Background())
	require.NoError(t, err)
	require.NotNil(t, keys)
	require.Equal(t, 2, calls)
}

func TestGetRekorPublicKeysWithRetry_DoesNotRetryOtherErrors(t *testing.T) {
	old := getRekorPublicKeys
	defer func() { getRekorPublicKeys = old }()

	permErr := errors.New("creating cached local store: permission denied")
	calls := 0
	getRekorPublicKeys = func(_ context.Context) (*cosign.TrustedTransparencyLogPubKeys, error) {
		calls++
		return nil, permErr
	}

	_, err := getRekorPublicKeysWithRetry(context.Background())
	require.ErrorIs(t, err, permErr)
	require.Equal(t, 1, calls)
}

func TestGetRekorPublicKeysWithRetry_ExhaustsRetriesOnPersistentLock(t *testing.T) {
	old := getRekorPublicKeys
	defer func() { getRekorPublicKeys = old }()

	lockErr := errors.New("creating cached local store: used by another process")
	calls := 0
	getRekorPublicKeys = func(_ context.Context) (*cosign.TrustedTransparencyLogPubKeys, error) {
		calls++
		return nil, lockErr
	}

	start := time.Now()
	_, err := getRekorPublicKeysWithRetry(context.Background())
	require.Error(t, err)
	require.Equal(t, rekorPubKeysRetryAttempts, calls)
	require.GreaterOrEqual(t, time.Since(start), rekorPubKeysRetryDelay*time.Duration(rekorPubKeysRetryAttempts-1))
}
