package repository

import (
	"context"
	"errors"
	"github.com/lib/pq"
	"log/slog"
	"strings"
	"time"
)

const (
	maxDatabaseInitializationRetries = 8
	databaseInitializationRetryBase  = time.Second
	databaseInitializationRetryMax   = 30 * time.Second
)

// initializeDatabaseWithRetry retries only errors that indicate PostgreSQL is
// temporarily unavailable during startup. Permanent configuration, migration,
// and data errors are returned immediately so they remain visible to operators.
func initializeDatabaseWithRetry(ctx context.Context, initialize func(context.Context) error) error {
	return initializeDatabaseWithRetryWithWait(ctx, initialize, waitForDatabaseInitializationRetry)
}

func initializeDatabaseWithRetryWithWait(
	ctx context.Context,
	initialize func(context.Context) error,
	wait func(context.Context, time.Duration) error,
) error {
	for attempt := 1; ; attempt++ {
		if err := initialize(ctx); err == nil {
			return nil
		} else {
			if !isTransientDatabaseInitializationError(err) || attempt > maxDatabaseInitializationRetries {
				return err
			}

			delay := databaseInitializationRetryBase * time.Duration(1<<(attempt-1))
			if delay > databaseInitializationRetryMax {
				delay = databaseInitializationRetryMax
			}
			slog.Warn("database initialization temporarily unavailable; retrying",
				"retry", attempt,
				"max_retries", maxDatabaseInitializationRetries,
				"retry_in", delay,
				"error", err,
			)
			if err := wait(ctx, delay); err != nil {
				return err
			}
		}
	}
}

func waitForDatabaseInitializationRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func isTransientDatabaseInitializationError(err error) bool {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) {
		return false
	}
	code := string(pqErr.Code)
	return code == "57P03" || strings.HasPrefix(code, "08")
}
