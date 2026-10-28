package oauth

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	seclient "github.com/docker/secrets-engine/client"
)

// TestStatusRetryDelay pins the backoff schedule a failed token-status read follows.
func TestStatusRetryDelay(t *testing.T) {
	expected := []time.Duration{30 * time.Second, 60 * time.Second, 2 * time.Minute, 4 * time.Minute}
	for i, want := range expected {
		if got := statusRetryDelay(i + 1); got != want {
			t.Errorf("attempt %d: got %v, want %v", i+1, got, want)
		}
	}
}

// TestNoTokenIsPermanent verifies the loop can tell "there is no credential" from a
// transient read failure, which is what decides whether it stops or retries.
func TestNoTokenIsPermanent(t *testing.T) {
	transient := errors.New("failed to query Secrets Engine for notion-remote: deadline_exceeded")
	if errors.Is(transient, ErrNoOAuthToken) {
		t.Fatal("a Secrets Engine timeout must not be classified as a missing token")
	}

	wrapped := fmt.Errorf("%w for %s", ErrNoOAuthToken, "notion-remote")
	if !errors.Is(wrapped, ErrNoOAuthToken) {
		t.Fatalf("a missing credential must wrap ErrNoOAuthToken, got %v", wrapped)
	}
	if wrapped.Error() != "no OAuth token stored for notion-remote" {
		t.Fatalf("unexpected message: %v", wrapped)
	}
}

// TestSleepIsInterruptible verifies a status retry stops promptly on shutdown and on
// context cancellation instead of sleeping out its backoff.
func TestSleepIsInterruptible(t *testing.T) {
	t.Run("stop channel", func(t *testing.T) {
		p := NewProvider("notion-remote", ModeCE, func(context.Context, string) error { return nil })
		done := make(chan bool, 1)
		go func() { done <- p.sleep(context.Background(), time.Hour) }()

		p.Stop()

		select {
		case keepGoing := <-done:
			if keepGoing {
				t.Fatal("a stopped provider must not continue its loop")
			}
		case <-time.After(5 * time.Second):
			t.Fatal("sleep ignored Stop")
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		p := NewProvider("notion-remote", ModeCE, func(context.Context, string) error { return nil })
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan bool, 1)
		go func() { done <- p.sleep(ctx, time.Hour) }()

		cancel()

		select {
		case keepGoing := <-done:
			if keepGoing {
				t.Fatal("a cancelled context must not continue the loop")
			}
		case <-time.After(5 * time.Second):
			t.Fatal("sleep ignored context cancellation")
		}
	})
}

// TestLoginEventResetsRetryCounters verifies a successful login clears the status retry
// budget as well as the refresh one, so a later blip gets the full schedule again.
func TestLoginEventResetsRetryCounters(t *testing.T) {
	reloaded := make(chan string, 1)
	p := NewProvider("notion-remote", ModeCE, func(_ context.Context, name string) error {
		reloaded <- name
		return nil
	})
	p.statusRetryCount = 2
	p.refreshRetryCount = 3
	p.lastRefreshExpiry = time.Now()

	go func() { p.SendEvent(Event{Type: EventLoginSuccess}) }()

	if !p.sleep(context.Background(), time.Hour) {
		t.Fatal("a login event must keep the loop running")
	}
	if p.statusRetryCount != 0 || p.refreshRetryCount != 0 {
		t.Fatalf("retry counters not reset: status=%d refresh=%d", p.statusRetryCount, p.refreshRetryCount)
	}
	if !p.lastRefreshExpiry.IsZero() {
		t.Fatal("lastRefreshExpiry not reset")
	}
	select {
	case name := <-reloaded:
		if name != "notion-remote" {
			t.Fatalf("reloaded %q, want notion-remote", name)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("login event did not reload the provider")
	}
}

// flakyStatus is a CredentialStatusReader that fails a fixed number of times and then
// reports a token that is valid for the rest of the hour.
type flakyStatus struct {
	failures int32
	calls    int32
}

func (f *flakyStatus) GetTokenStatus(context.Context, seclient.ID) (TokenStatus, error) {
	atomic.AddInt32(&f.calls, 1)
	if atomic.AddInt32(&f.failures, -1) >= 0 {
		return TokenStatus{}, errors.New("deadline_exceeded: Post \"http://unix/resolver.v1.ResolverService/GetSecrets\"")
	}
	return TokenStatus{Valid: true, ExpiresAt: time.Now().Add(time.Hour)}, nil
}

// TestRunSurvivesATransientStatusFailure drives Run itself: a status read that times out
// must not end the loop, which is the reported failure mode of #601.
func TestRunSurvivesATransientStatusFailure(t *testing.T) {
	status := &flakyStatus{failures: 2}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := NewProvider("notion-remote", ModeCE, func(context.Context, string) error { return nil })
	p.credHelper = status
	p.retryDelay = func(int) time.Duration { return time.Millisecond }

	done := make(chan struct{})
	go func() {
		p.Run(ctx)
		close(done)
	}()

	// Three successful reads are what a resilient loop needs: the two failures, then at
	// least one more pass through the status check.
	deadline := time.After(10 * time.Second)
	for atomic.LoadInt32(&status.calls) < 3 {
		select {
		case <-deadline:
			t.Fatalf("loop stopped after %d status reads; a transient failure ended it", atomic.LoadInt32(&status.calls))
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after cancellation")
	}
}

// TestRunStopsWhenNoTokenIsStored verifies the loop still stops immediately for a server
// that was never authorized, instead of retrying a condition that cannot change.
func TestRunStopsWhenNoTokenIsStored(t *testing.T) {
	status := &alwaysMissing{}
	p := NewProvider("notion-remote", ModeCE, func(context.Context, string) error { return nil })
	p.credHelper = status
	p.retryDelay = func(int) time.Duration { return time.Millisecond }

	done := make(chan struct{})
	go func() {
		p.Run(context.Background())
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run kept retrying a server with no stored token")
	}
	if calls := atomic.LoadInt32(&status.calls); calls != 1 {
		t.Fatalf("expected a single status read, got %d", calls)
	}
}

type alwaysMissing struct {
	calls int32
}

func (a *alwaysMissing) GetTokenStatus(context.Context, seclient.ID) (TokenStatus, error) {
	atomic.AddInt32(&a.calls, 1)
	return TokenStatus{}, fmt.Errorf("%w for notion-remote", ErrNoOAuthToken)
}
