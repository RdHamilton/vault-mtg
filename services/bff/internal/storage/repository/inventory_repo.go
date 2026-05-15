package repository

import (
	"context"
	"time"
)

// InventoryUpsert holds the fields written to the inventory table from an
// inventory.updated daemon event.  AccountID is the resolved accounts.id
// BIGINT FK (migration 000080 converts the column from TEXT client_id to
// BIGINT FK so every write is properly tenant-scoped).
type InventoryUpsert struct {
	AccountID          int64
	Gems               int
	Gold               int
	TotalVaultProgress int
	WildCardCommons    int
	WildCardUncommons  int
	WildCardRares      int
	WildCardMythics    int
	UpdatedAt          time.Time
}

// InventoryRepository writes player inventory snapshots to the inventory table.
type InventoryRepository struct {
	db DB
}

// NewInventoryRepository returns an InventoryRepository backed by db.
func NewInventoryRepository(db DB) *InventoryRepository {
	return &InventoryRepository{db: db}
}

// UpsertInventory writes an inventory snapshot for the given account.
// The inventory table has one row per account_id; subsequent writes update all
// tracked fields in-place.  The inventory table was originally designed as a
// singleton row (migration 000023) and later had account_id added (migration
// 000068).  The ON CONFLICT clause keys on account_id so that each account
// maintains its own current snapshot.
func (r *InventoryRepository) UpsertInventory(ctx context.Context, u InventoryUpsert) error {
	const q = `
		INSERT INTO inventory (
			account_id,
			gold,
			gems,
			wc_common,
			wc_uncommon,
			wc_rare,
			wc_mythic,
			vault_progress,
			draft_tokens,
			sealed_tokens,
			updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (account_id) DO UPDATE
			SET gold          = EXCLUDED.gold,
			    gems          = EXCLUDED.gems,
			    wc_common     = EXCLUDED.wc_common,
			    wc_uncommon   = EXCLUDED.wc_uncommon,
			    wc_rare       = EXCLUDED.wc_rare,
			    wc_mythic     = EXCLUDED.wc_mythic,
			    vault_progress = EXCLUDED.vault_progress,
			    updated_at   = EXCLUDED.updated_at`

	_, err := r.db.ExecContext(
		ctx, q,
		u.AccountID,
		u.Gold,
		u.Gems,
		u.WildCardCommons,
		u.WildCardUncommons,
		u.WildCardRares,
		u.WildCardMythics,
		u.TotalVaultProgress,
		0, // draft_tokens — not in InventoryUpdatedPayload; preserve existing via ON CONFLICT
		0, // sealed_tokens — same
		u.UpdatedAt,
	)

	return err
}
