// internal/importer/retry.go
package importer

import (
	"context"
	"fmt"
	"time"
)

func (i *Importer) fetchWithRetry(ctx context.Context, url string, maxRetries int) (*APIResponse, error) {
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		data, err := i.fetchData(ctx, url)
		if err == nil {
			return data, nil
		}

		lastErr = err

		// Exponential backoff
		backoff := time.Duration(1<<uint(attempt)) * time.Second

		i.logger.Warn("Request failed, retrying",
			"attempt", attempt+1,
			"max_retries", maxRetries,
			"backoff", backoff.String(),
			"error", err,
		)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}
