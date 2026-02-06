// Package idempotency provides mechanisms to prevent duplicate webhook processing.
package idempotency

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore is a PostgreSQL implementation of Store.
type PostgresStore struct {
	db *pgxpool.Pool
}

// NewPostgresStore creates a new PostgreSQL-backed idempotency store.
func NewPostgresStore(db *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{db: db}
}

// CheckAndSet implements Store.CheckAndSet for PostgreSQL storage.
// Uses INSERT ... ON CONFLICT to atomically check and set the key.
func (s *PostgresStore) CheckAndSet(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	expiresAt := time.Now().Add(ttl)

	// Try to insert the key. If it already exists and hasn't expired, the INSERT will fail.
	// If it exists but is expired, we update it.
	query := `
		INSERT INTO idempotency_keys (key, expires_at)
		VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE
		SET expires_at = EXCLUDED.expires_at, created_at = NOW()
		WHERE idempotency_keys.expires_at < NOW()
		RETURNING key
	`

	var returnedKey string
	err := s.db.QueryRow(ctx, query, key, expiresAt).Scan(&returnedKey)

	if err != nil {
		// No rows returned means the key exists and hasn't expired (duplicate request)
		if err.Error() == "no rows in result set" {
			return false, nil
		}
		return false, err
	}

	// Key was inserted or updated (expired key was replaced)
	return true, nil
}

// Delete implements Store.Delete for PostgreSQL storage.
func (s *PostgresStore) Delete(ctx context.Context, key string) error {
	_, err := s.db.Exec(ctx, "DELETE FROM idempotency_keys WHERE key = $1", key)
	return err
}

// Cleanup removes all expired keys from the database.
// This should be called periodically by a background job.
func (s *PostgresStore) Cleanup(ctx context.Context) (int64, error) {
	result, err := s.db.Exec(ctx, "DELETE FROM idempotency_keys WHERE expires_at < NOW()")
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}
