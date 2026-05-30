package database

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KistametL/WMS/backend/internal/config"
)

func NewPool(cfg *config.Config) (*pgxpool.Pool, error) {
	// Build the DSN as a URL so net/url handles percent-encoding automatically.
	// Using fmt.Sprintf with key=value format would break if the password
	// contains spaces, single-quotes, or backslashes (PostgreSQL libpq spec).
	u := &url.URL{
		Scheme: "postgresql",
		User:   url.UserPassword(cfg.Database.User, cfg.Database.Password),
		Host:   cfg.Database.Host + ":" + cfg.Database.Port,
		Path:   "/" + cfg.Database.Name,
	}
	q := url.Values{}
	q.Set("sslmode", cfg.Database.SSLMode)
	u.RawQuery = q.Encode()

	pool, err := pgxpool.New(context.Background(), u.String())
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Use a deadline on the startup ping — without one, an unreachable database
	// would block the process indefinitely and the app would never become ready.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		pool.Close() // prevent background health-check goroutine from leaking
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}
