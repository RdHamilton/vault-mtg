package repository

import (
	"context"
	"time"
)

// CardInventoryUpsert holds the data needed to apply one card entry from a
// collection delta to the card_inventory table.
type CardInventoryUpsert struct {
	AccountID    int64
	CardID       int
	Count        int
	SnapshotHash string
}

// CardInventoryRow is returned when reading a card_inventory row.
type CardInventoryRow struct {
	ID           int64
	AccountID    int64
	CardID       int
	Count        int
	SnapshotHash string
	UpdatedAt    time.Time
}

// CardInventoryRepository provides write and read access to card_inventory
// scoped by account_id.
type CardInventoryRepository struct {
	db DB
}

// NewCardInventoryRepository returns a CardInventoryRepository backed by db.
func NewCardInventoryRepository(db DB) *CardInventoryRepository {
	return &CardInventoryRepository{db: db}
}

// UpsertDelta applies a single card delta entry to the card_inventory table.
//
// Idempotency: the unique index on (account_id, card_id, snapshot_hash) means
// that a row for the same (account_id, card_id, snapshot_hash) triple is
// silently skipped on conflict.  A different snapshot_hash for the same
// (account_id, card_id) replaces the count and updates updated_at.
//
// The operation is scoped strictly to account_id — no cross-tenant writes
// are possible.
func (r *CardInventoryRepository) UpsertDelta(ctx context.Context, u CardInventoryUpsert) error {
	const q = `
		INSERT INTO card_inventory (account_id, card_id, count, snapshot_hash, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (account_id, card_id)
		DO UPDATE SET
			count         = EXCLUDED.count,
			snapshot_hash = EXCLUDED.snapshot_hash,
			updated_at    = NOW()
		WHERE card_inventory.snapshot_hash <> EXCLUDED.snapshot_hash`

	_, err := r.db.ExecContext(ctx, q, u.AccountID, u.CardID, u.Count, u.SnapshotHash)
	return err
}

// ListByAccountIDCursor returns up to limit+1 card_inventory rows using keyset
// pagination ordered by card_id ASC. The extra row signals has_more=true.
//
// afterCardID is the card_id from the last row of the previous page (0 on the
// first page, which starts from card_id=1).
func (r *CardInventoryRepository) ListByAccountIDCursor(
	ctx context.Context,
	accountID int64,
	afterCardID int,
	limit int,
) ([]CardInventoryRow, error) {
	fetch := limit + 1

	var q string
	var args []interface{}

	if afterCardID > 0 {
		q = `
			SELECT id, account_id, card_id, count, snapshot_hash, updated_at
			FROM card_inventory
			WHERE account_id = $1 AND card_id > $2
			ORDER BY card_id ASC
			LIMIT $3`

		args = []interface{}{accountID, afterCardID, fetch}
	} else {
		q = `
			SELECT id, account_id, card_id, count, snapshot_hash, updated_at
			FROM card_inventory
			WHERE account_id = $1
			ORDER BY card_id ASC
			LIMIT $2`

		args = []interface{}{accountID, fetch}
	}

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	var cards []CardInventoryRow

	for rows.Next() {
		var c CardInventoryRow
		if err := rows.Scan(
			&c.ID, &c.AccountID, &c.CardID, &c.Count, &c.SnapshotHash, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}

		cards = append(cards, c)
	}

	return cards, rows.Err()
}

// GetByAccountAndCard retrieves a single card_inventory row for the given
// (account_id, card_id) pair.  Returns sql.ErrNoRows when no row exists.
func (r *CardInventoryRepository) GetByAccountAndCard(ctx context.Context, accountID int64, cardID int) (CardInventoryRow, error) {
	const q = `
		SELECT id, account_id, card_id, count, snapshot_hash, updated_at
		FROM card_inventory
		WHERE account_id = $1 AND card_id = $2`

	var row CardInventoryRow
	err := r.db.QueryRowContext(ctx, q, accountID, cardID).Scan(
		&row.ID,
		&row.AccountID,
		&row.CardID,
		&row.Count,
		&row.SnapshotHash,
		&row.UpdatedAt,
	)
	return row, err
}
