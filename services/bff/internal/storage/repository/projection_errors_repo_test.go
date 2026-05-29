package repository_test

import (
	"context"
	"testing"

	"github.com/RdHamilton/vault-mtg/services/bff/internal/storage/repository"
)

// TestProjectionErrorsRepository_Insert writes a dead-letter row and verifies
// it is retrievable from the database (integration, requires TEST_DATABASE_URL).
func TestProjectionErrorsRepository_Insert(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewProjectionErrorsRepository(db)

	ins := repository.ProjectionErrorInsert{
		DaemonEventID: 99999,
		AccountID:     "test-acct-dlq-001",
		EventType:     "match.completed",
		RawPayload:    `{"match_id":"bad-row"}`,
		ErrorMessage:  "match payload missing format",
	}

	err := repo.Insert(context.Background(), ins)
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(
			context.Background(),
			`DELETE FROM projection_errors WHERE account_id = 'test-acct-dlq-001'`,
		)
	})

	// Verify the row landed.
	var count int
	row := db.QueryRowContext(
		context.Background(),
		`SELECT COUNT(*) FROM projection_errors WHERE account_id = $1 AND daemon_event_id = $2`,
		"test-acct-dlq-001", int64(99999),
	)
	if err := row.Scan(&count); err != nil {
		t.Fatalf("SELECT COUNT: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 row, got %d", count)
	}
}

// TestProjectionErrorsRepository_Insert_NonJSONPayload verifies that a
// non-JSON raw_payload does not cause a Postgres-level error — this is the
// key reason raw_payload is TEXT rather than JSONB (ADR-039 deviation).
func TestProjectionErrorsRepository_Insert_NonJSONPayload(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewProjectionErrorsRepository(db)

	ins := repository.ProjectionErrorInsert{
		DaemonEventID: 99998,
		AccountID:     "test-acct-dlq-nonjson",
		EventType:     "match.completed",
		RawPayload:    `not valid JSON at all: {broken`,
		ErrorMessage:  "unmarshal match payload: invalid character 'n' looking for beginning of value",
	}

	err := repo.Insert(context.Background(), ins)
	if err != nil {
		t.Fatalf("Insert with non-JSON payload: %v — raw_payload column should be TEXT not JSONB", err)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(
			context.Background(),
			`DELETE FROM projection_errors WHERE account_id = 'test-acct-dlq-nonjson'`,
		)
	})
}
