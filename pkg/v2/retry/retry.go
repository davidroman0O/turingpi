// Package retry provides retry mechanisms with exponential backoff
package retry

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// Operation represents a function that can be retried
type Operation func(ctx context.Context) error

// Config holds retry configuration
type Config struct {
	// MaxAttempts is the maximum number of attempts including the initial attempt
	MaxAttempts int

	// InitialDelay is the delay before the first retry
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration

	// Multiplier is the factor by which the delay increases
	Multiplier float64

	// MaxJitter is the maximum random jitter added to delays
	MaxJitter time.Duration

	// OnRetry is called after each retry attempt
	OnRetry func(attempt int, err error)
}

// DefaultConfig returns a default retry configuration
func DefaultConfig() Config {
	return Config{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		MaxJitter:    100 * time.Millisecond,
	}
}

// WithBackoff retries an operation with exponential backoff
func WithBackoff(ctx context.Context, op Operation, cfg Config) error {
	var lastErr error

	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return fmt.Errorf("operation cancelled: %w", ctx.Err())
		default:
		}

		// If this isn't the first attempt, wait with backoff
		if attempt > 0 {
			delay := calculateDelay(attempt, cfg)
			timer := time.NewTimer(delay)

			select {
			case <-ctx.Done():
				timer.Stop()
				return fmt.Errorf("operation cancelled during backoff: %w", ctx.Err())
			case <-timer.C:
			}
		}

		// Attempt the operation
		err := op(ctx)
		if err == nil {
			return nil
		}

		lastErr = err

		// Call OnRetry callback if configured
		if cfg.OnRetry != nil {
			cfg.OnRetry(attempt+1, err)
		}

		// Check if error is not retryable
		if !IsRetryable(err) {
			return fmt.Errorf("non-retryable error: %w", err)
		}
	}

	return fmt.Errorf("operation failed after %d attempts: %w", cfg.MaxAttempts, lastErr)
}

// calculateDelay calculates the delay for a given attempt
func calculateDelay(attempt int, cfg Config) time.Duration {
	// Calculate base delay with exponential backoff
	delay := float64(cfg.InitialDelay) * math.Pow(cfg.Multiplier, float64(attempt))

	// Apply maximum delay limit
	if delay > float64(cfg.MaxDelay) {
		delay = float64(cfg.MaxDelay)
	}

	// Add random jitter
	if cfg.MaxJitter > 0 {
		jitter := float64(cfg.MaxJitter)
		delay += float64(time.Duration(float64(time.Nanosecond) * jitter * rand.Float64()))
	}

	return time.Duration(delay)
}

// RetryableError is an error that can be retried
type RetryableError struct {
	err error
}

// Error implements the error interface
func (e *RetryableError) Error() string {
	return fmt.Sprintf("retryable error: %v", e.err)
}

// Unwrap returns the underlying error
func (e *RetryableError) Unwrap() error {
	return e.err
}

// NewRetryableError wraps an error as retryable
func NewRetryableError(err error) error {
	if err == nil {
		return nil
	}
	return &RetryableError{err: err}
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	var retryable *RetryableError
	return errors.As(err, &retryable)
}

// WithRetryable wraps an operation to make its errors retryable
func WithRetryable(op Operation) Operation {
	return func(ctx context.Context) error {
		err := op(ctx)
		if err != nil {
			return NewRetryableError(err)
		}
		return nil
	}
}
