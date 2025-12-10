package utils

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRetry_SucceedsImmediately(t *testing.T) {
	calls := 0
	err := Retry(3, time.Millisecond, func() error {
		calls++
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 1, calls)
}

func TestRetry_SucceedsAfterRetries(t *testing.T) {
	calls := 0
	err := Retry(3, time.Millisecond, func() error {
		calls++
		if calls < 3 {
			return errors.New("fail")
		}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 3, calls)
}

func TestRetry_ExhaustsAttempts(t *testing.T) {
	calls := 0
	err := Retry(3, time.Millisecond, func() error {
		calls++
		return errors.New("always fail")
	})
	require.Error(t, err)
	require.Equal(t, 3, calls)
}

func TestRetryIfErrorIs_RetriesOnMatchingError(t *testing.T) {
	targetErr := errors.New("retryable")
	calls := 0
	err := RetryIfErrorIs(3, time.Millisecond, func() error {
		calls++
		if calls < 2 {
			return targetErr
		}
		return nil
	}, targetErr)
	require.NoError(t, err)
	require.Equal(t, 2, calls)
}

func TestRetryIfErrorIs_StopsOnNonMatchingError(t *testing.T) {
	targetErr := errors.New("retryable")
	otherErr := errors.New("other")
	calls := 0
	err := RetryIfErrorIs(3, time.Millisecond, func() error {
		calls++
		return otherErr
	}, targetErr)
	require.ErrorIs(t, err, otherErr)
	require.Equal(t, 1, calls)
}

func TestRetryIfErrorIs_ExhaustsAttempts(t *testing.T) {
	targetErr := errors.New("retryable")
	calls := 0
	err := RetryIfErrorIs(3, time.Millisecond, func() error {
		calls++
		return targetErr
	}, targetErr)
	require.ErrorIs(t, err, targetErr)
	require.Equal(t, 3, calls)
}
