package resilience

import (
	"context"
	"fmt"
	"time"
)

type RetryConfig struct {
	MaxAttempts int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
}

func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 5,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
	}
}

func RetryWithContext(ctx context.Context, config RetryConfig, fn func() error) error {
	var lastErr error
	delay := config.InitialDelay

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err
		
		if attempt < config.MaxAttempts {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				delay = time.Duration(float64(delay) * config.Multiplier)
				if delay > config.MaxDelay {
					delay = config.MaxDelay
				}
			}
		}
	}

	return fmt.Errorf("retry failed after %d attempts: %w", config.MaxAttempts, lastErr)
}