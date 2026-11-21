package signatures

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/sigstore/cosign/v2/pkg/cosign"

	"github.com/docker/mcp-gateway/pkg/retry"
)

// getRekorPublicKeys opens the sigstore TUF cache (LevelDB at ~/.sigstore/root/tuf.db) via cosign.
var getRekorPublicKeys = cosign.GetRekorPubs

// rekorPubKeysMu avoids concurrent TUF cache opens from multiple goroutines in one process.
var rekorPubKeysMu sync.Mutex

const (
	rekorPubKeysRetryAttempts = 8
	rekorPubKeysRetryDelay    = 200 * time.Millisecond
)

func getRekorPublicKeysWithRetry(ctx context.Context) (*cosign.TrustedTransparencyLogPubKeys, error) {
	rekorPubKeysMu.Lock()
	defer rekorPubKeysMu.Unlock()

	var keys *cosign.TrustedTransparencyLogPubKeys
	err := retry.If(rekorPubKeysRetryAttempts, rekorPubKeysRetryDelay, func() error {
		var err error
		keys, err = getRekorPublicKeys(ctx)
		return err
	}, isTUFCachedStoreLockError)
	if err != nil {
		return nil, err
	}
	return keys, nil
}

func isTUFCachedStoreLockError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "creating cached local store") {
		return false
	}
	if strings.Contains(msg, "used by another process") {
		return true
	}
	if strings.Contains(msg, "resource temporarily unavailable") {
		return true
	}
	return strings.Contains(msg, "lock")
}
