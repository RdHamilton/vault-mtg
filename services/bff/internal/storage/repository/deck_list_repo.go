package repository

import (
	"context"
	"database/sql"
	"time"
)

// DeckRow is a row returned from the decks table for list reads.
type DeckRow struct {
	ID         string
	Name       string
	Format     string
	Source     string
	ModifiedAt time.Time
}

// DeckListRepository provides read access to the decks table scoped by account_id.
type DeckListRepository struct {
	db DB
}

// NewDeckListRepository returns a DeckListRepository backed by db.
func NewDeckListRepository(db DB) *DeckListRepository {
	return &DeckListRepository{db: db}
}

// ListByAccountIDCursor returns up to limit+1 deck rows using keyset (cursor)
// pagination ordered by modified_at DESC, id DESC. The extra row signals
// has_more=true to the caller.
//
// When cursorModifiedAt is non-nil the keyset predicate
// (modified_at < cursorModifiedAt OR (modified_at = cursorModifiedAt AND id < cursorID))
// is applied. format may be empty to return decks in all formats.
func (r *DeckListRepository) ListByAccountIDCursor(
	ctx context.Context,
	accountID int64,
	format string,
	cursorModifiedAt *time.Time,
	cursorID string,
	limit int,
) ([]DeckRow, error) {
	fetch := limit + 1

	var (
		rows *sql.Rows
		err  error
	)

	switch {
	case format != "" && cursorModifiedAt != nil:
		const q = `
			SELECT id, name, format, COALESCE(source, 'arena') AS source, modified_at
			FROM decks
			WHERE account_id = $1
			  AND lower(format) = lower($2)
			  AND (modified_at < $3 OR (modified_at = $3 AND id < $4))
			ORDER BY modified_at DESC, id DESC
			LIMIT $5`

		rows, err = r.db.QueryContext(ctx, q, accountID, format, *cursorModifiedAt, cursorID, fetch)

	case format != "" && cursorModifiedAt == nil:
		const q = `
			SELECT id, name, format, COALESCE(source, 'arena') AS source, modified_at
			FROM decks
			WHERE account_id = $1 AND lower(format) = lower($2)
			ORDER BY modified_at DESC, id DESC
			LIMIT $3`

		rows, err = r.db.QueryContext(ctx, q, accountID, format, fetch)

	case format == "" && cursorModifiedAt != nil:
		const q = `
			SELECT id, name, format, COALESCE(source, 'arena') AS source, modified_at
			FROM decks
			WHERE account_id = $1
			  AND (modified_at < $2 OR (modified_at = $2 AND id < $3))
			ORDER BY modified_at DESC, id DESC
			LIMIT $4`

		rows, err = r.db.QueryContext(ctx, q, accountID, *cursorModifiedAt, cursorID, fetch)

	default:
		const q = `
			SELECT id, name, format, COALESCE(source, 'arena') AS source, modified_at
			FROM decks
			WHERE account_id = $1
			ORDER BY modified_at DESC, id DESC
			LIMIT $2`

		rows, err = r.db.QueryContext(ctx, q, accountID, fetch)
	}

	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	var decks []DeckRow

	for rows.Next() {
		var d DeckRow
		if err := rows.Scan(&d.ID, &d.Name, &d.Format, &d.Source, &d.ModifiedAt); err != nil {
			return nil, err
		}

		decks = append(decks, d)
	}

	return decks, rows.Err()
}
