package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// User is the in-memory representation of a row in the users table.
type User struct {
	ID               int64
	Email            string
	ClerkUserID      *string
	SubscriptionTier string
	CreatedAt        time.Time
}

// UserRepository handles persistence for users rows.
type UserRepository struct {
	db DB
}

// NewUserRepository returns a UserRepository backed by db.
func NewUserRepository(db DB) *UserRepository {
	return &UserRepository{db: db}
}

// GetByClerkUserID returns the user whose clerk_user_id matches, or (nil, nil) if not found.
func (r *UserRepository) GetByClerkUserID(ctx context.Context, clerkUserID string) (*User, error) {
	const q = `
		SELECT id, email, clerk_user_id, subscription_tier, created_at
		FROM   users
		WHERE  clerk_user_id = $1`

	row := r.db.QueryRowContext(ctx, q, clerkUserID)

	var u User
	if err := row.Scan(&u.ID, &u.Email, &u.ClerkUserID, &u.SubscriptionTier, &u.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, fmt.Errorf("GetByClerkUserID: %w", err)
	}

	return &u, nil
}

// UpsertByClerkUserID inserts a new user row if clerk_user_id is not known, or returns the
// existing one.  For JIT provisioning the email placeholder "<clerkUserID>@clerk.local" is
// used on insert; it is overwritten when the user provides a real email later.
func (r *UserRepository) UpsertByClerkUserID(ctx context.Context, clerkUserID string) (*User, error) {
	// Use an INSERT … ON CONFLICT DO NOTHING pattern combined with a SELECT so
	// we always return the canonical row regardless of whether we just created it.
	const q = `
		INSERT INTO users (email, clerk_user_id, subscription_tier)
		VALUES ($1, $2, 'free')
		ON CONFLICT (clerk_user_id) WHERE clerk_user_id IS NOT NULL DO NOTHING
		RETURNING id, email, clerk_user_id, subscription_tier, created_at`

	placeholder := clerkUserID + "@clerk.local"
	row := r.db.QueryRowContext(ctx, q, placeholder, clerkUserID)

	var u User
	if err := row.Scan(&u.ID, &u.Email, &u.ClerkUserID, &u.SubscriptionTier, &u.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Row already exists — fetch it.
			return r.GetByClerkUserID(ctx, clerkUserID)
		}

		return nil, fmt.Errorf("UpsertByClerkUserID: %w", err)
	}

	return &u, nil
}
