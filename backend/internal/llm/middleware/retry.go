package middleware

import (
	"context"
	"fmt"
	"time"
)

// RetryConfig — конфигурация retry
type RetryConfig struct {
	MaxAttempts  int           
	InitialDelay time.Duration 
	MaxDelay     time.Duration 
	Multiplier   float64       
}

// DefaultRetryConfig — конфигурация по умолчанию
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:  2,
		InitialDelay: 500 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
	}
}

type RetryableFunc func() error

func DoWithRetry(ctx context.Context, fn RetryableFunc, config RetryConfig) error {
	var lastErr error
	delay := config.InitialDelay

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled before attempt %d: %w", attempt, ctx.Err())
		default:
		}

		if err := fn(); err == nil {
			return nil
		} else {
			lastErr = err
		}

		if attempt == config.MaxAttempts {
			break
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled during retry delay: %w", ctx.Err())
		case <-time.After(delay):
		}

		delay = time.Duration(float64(delay) * config.Multiplier)
		if delay > config.MaxDelay {
			delay = config.MaxDelay
		}
	}

	return fmt.Errorf("failed after %d attempts: %w", config.MaxAttempts, lastErr)
}

// RetryableGenerate — обертка для LLM генерации с retry
type RetryableGenerate struct {
	client interface {
		Generate(ctx context.Context, prompt string) (string, error)
	}
	config RetryConfig
}

// NewRetryableGenerate создает обертку с retry
func NewRetryableGenerate(client interface {
	Generate(ctx context.Context, prompt string) (string, error)
}, config RetryConfig) *RetryableGenerate {
	return &RetryableGenerate{
		client: client,
		config: config,
	}
}

// Generate выполняет генерацию с повторами
func (r *RetryableGenerate) Generate(ctx context.Context, prompt string) (string, error) {
	var result string
	var lastErr error

	err := DoWithRetry(ctx, func() error {
		var err error
		result, err = r.client.Generate(ctx, prompt)
		if err != nil {
			lastErr = err
			return err
		}
		return nil
	}, r.config)

	if err != nil {
		return "", fmt.Errorf("retryable generate failed: %w", lastErr)
	}
	return result, nil
}
