package retry

import (
	"errors"
	"time"
)

func Retry(attempts int, sleep time.Duration, fn func() error) error {
	return If(attempts, sleep, fn, func(err error) bool {
		return err != nil
	})
}

func IfErrorIs(attempts int, sleep time.Duration, fn func() error, target error) error {
	return If(attempts, sleep, fn, func(err error) bool {
		return errors.Is(err, target)
	})
}

func If(attempts int, sleep time.Duration, fn func() error, predicate func(error) bool) (err error) {
	for i := range attempts {
		if err = fn(); err == nil {
			return nil
		}
		if !predicate(err) || i >= attempts-1 {
			break
		}
		time.Sleep(sleep)
	}
	return err
}
