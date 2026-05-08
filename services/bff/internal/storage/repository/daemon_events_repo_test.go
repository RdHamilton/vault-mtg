package repository_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

func TestDaemonEventsRepository_Insert_NoError(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDaemonEventsRepository(db)

	payload := json.RawMessage(`{"key":"value"}`)
	occurredAt := time.Now().UTC().Truncate(time.Second)

	err := repo.Insert(context.Background(), 1, "test-account-1", "match.game_started", payload, occurredAt, "", 0)
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	// Cleanup the inserted row.
	t.Cleanup(func() {
		_, _ = db.ExecContext(
			context.Background(),
			`DELETE FROM daemon_events WHERE user_id = 1 AND account_id = 'test-account-1' AND event_type = 'match.game_started'`,
		)
	})
}

func TestDaemonEventsRepository_Insert_WithEventID(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDaemonEventsRepository(db)

	payload := json.RawMessage(`{"key":"value"}`)
	occurredAt := time.Now().UTC().Truncate(time.Second)
	eventID := "evt_test_001"

	err := repo.Insert(context.Background(), 1, "test-account-eventid", "match.completed", payload, occurredAt, eventID, 0)
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(
			context.Background(),
			`DELETE FROM daemon_events WHERE account_id = 'test-account-eventid'`,
		)
	})

	// Verify idempotency: second insert with same event_id must be a no-op.
	err = repo.Insert(context.Background(), 1, "test-account-eventid", "match.completed", payload, occurredAt, eventID, 0)
	if err != nil {
		t.Fatalf("idempotent Insert: %v", err)
	}

	rows, err := repo.ListByUserID(context.Background(), 1, 100)
	if err != nil {
		t.Fatalf("ListByUserID: %v", err)
	}

	count := 0
	for _, r := range rows {
		if r.AccountID == "test-account-eventid" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 row for idempotent insert, got %d", count)
	}
}

func TestDaemonEventsRepository_ListByUserID_OrderedNewestFirst(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDaemonEventsRepository(db)

	const userID int64 = 9991
	const accountID = "test-account-ordered"

	older := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	newer := time.Now().UTC().Add(-1 * time.Hour).Truncate(time.Second)

	payload := json.RawMessage(`{"seq":1}`)

	if err := repo.Insert(context.Background(), userID, accountID, "event.a", payload, older, "", 1); err != nil {
		t.Fatalf("Insert older: %v", err)
	}

	payload2 := json.RawMessage(`{"seq":2}`)

	if err := repo.Insert(context.Background(), userID, accountID, "event.b", payload2, newer, "", 2); err != nil {
		t.Fatalf("Insert newer: %v", err)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(
			context.Background(),
			`DELETE FROM daemon_events WHERE user_id = $1 AND account_id = $2`,
			userID, accountID,
		)
	})

	events, err := repo.ListByUserID(context.Background(), userID, 10)
	if err != nil {
		t.Fatalf("ListByUserID: %v", err)
	}

	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}

	// Newest first — first element should have the newer occurred_at.
	if !events[0].OccurredAt.Equal(newer) {
		t.Errorf("expected first event occurred_at=%v, got %v", newer, events[0].OccurredAt)
	}

	if !events[1].OccurredAt.Equal(older) {
		t.Errorf("expected second event occurred_at=%v, got %v", older, events[1].OccurredAt)
	}
}

func TestDaemonEventsRepository_ListByUserID_ScopedToUser(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDaemonEventsRepository(db)

	const userA int64 = 9992
	const userB int64 = 9993
	const accountA = "test-account-a"
	const accountB = "test-account-b"

	occurredAt := time.Now().UTC().Truncate(time.Second)
	payload := json.RawMessage(`{}`)

	if err := repo.Insert(context.Background(), userA, accountA, "event.x", payload, occurredAt, "", 0); err != nil {
		t.Fatalf("Insert userA: %v", err)
	}

	if err := repo.Insert(context.Background(), userB, accountB, "event.y", payload, occurredAt, "", 0); err != nil {
		t.Fatalf("Insert userB: %v", err)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(
			context.Background(),
			`DELETE FROM daemon_events WHERE user_id IN ($1, $2)`,
			userA, userB,
		)
	})

	eventsA, err := repo.ListByUserID(context.Background(), userA, 10)
	if err != nil {
		t.Fatalf("ListByUserID userA: %v", err)
	}

	for _, e := range eventsA {
		if e.UserID != userA {
			t.Errorf("expected only userA events, got user_id=%d", e.UserID)
		}
	}

	eventsB, err := repo.ListByUserID(context.Background(), userB, 10)
	if err != nil {
		t.Fatalf("ListByUserID userB: %v", err)
	}

	for _, e := range eventsB {
		if e.UserID != userB {
			t.Errorf("expected only userB events, got user_id=%d", e.UserID)
		}
	}
}

func TestDaemonEventsRepository_HasRecentEvent_Connected(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDaemonEventsRepository(db)

	const userID int64 = 9994
	const accountID = "test-account-health-connected"

	payload := json.RawMessage(`{}`)
	occurredAt := time.Now().UTC().Truncate(time.Second)

	if err := repo.Insert(context.Background(), userID, accountID, "heartbeat", payload, occurredAt, "", 0); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(
			context.Background(),
			`DELETE FROM daemon_events WHERE user_id = $1 AND account_id = $2`,
			userID, accountID,
		)
	})

	connected, err := repo.HasRecentEventByUserID(context.Background(), userID, 60*time.Second)
	if err != nil {
		t.Fatalf("HasRecentEventByUserID: %v", err)
	}

	if !connected {
		t.Error("expected connected=true for a row inserted just now")
	}
}

func TestDaemonEventsRepository_HasRecentEvent_Disconnected_NoRows(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDaemonEventsRepository(db)

	// Use a user ID that has no rows in the test database.
	const userID int64 = 9995

	connected, err := repo.HasRecentEventByUserID(context.Background(), userID, 60*time.Second)
	if err != nil {
		t.Fatalf("HasRecentEventByUserID: %v", err)
	}

	if connected {
		t.Error("expected connected=false for a user with no daemon_events rows")
	}
}

func TestDaemonEventsRepository_HasRecentEvent_Disconnected_OldRow(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDaemonEventsRepository(db)

	const userID int64 = 9996
	const accountID = "test-account-health-old"

	payload := json.RawMessage(`{}`)
	// occurred_at is fine being in the past; we need received_at to be old.
	// We insert directly with an explicit old received_at to simulate a stale row.
	occurredAt := time.Now().UTC().Add(-5 * time.Minute).Truncate(time.Second)

	_, err := db.ExecContext(
		context.Background(),
		`INSERT INTO daemon_events (user_id, account_id, event_type, payload, occurred_at, received_at)
		 VALUES ($1, $2, $3, $4, $5, NOW() - INTERVAL '5 minutes')`,
		userID, accountID, "heartbeat", payload, occurredAt,
	)
	if err != nil {
		t.Fatalf("direct insert: %v", err)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(
			context.Background(),
			`DELETE FROM daemon_events WHERE user_id = $1 AND account_id = $2`,
			userID, accountID,
		)
	})

	connected, err := repo.HasRecentEventByUserID(context.Background(), userID, 60*time.Second)
	if err != nil {
		t.Fatalf("HasRecentEventByUserID: %v", err)
	}

	if connected {
		t.Error("expected connected=false for a row older than the window")
	}
}

func TestDaemonEventsRepository_HasRecentEvent_ScopedToUser(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDaemonEventsRepository(db)

	// User A has a recent row; user B must not see it as connected.
	const userA int64 = 9997
	const userB int64 = 9998
	const accountA = "test-account-health-a"

	payload := json.RawMessage(`{}`)
	occurredAt := time.Now().UTC().Truncate(time.Second)

	if err := repo.Insert(context.Background(), userA, accountA, "heartbeat", payload, occurredAt, "", 0); err != nil {
		t.Fatalf("Insert userA: %v", err)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(
			context.Background(),
			`DELETE FROM daemon_events WHERE user_id IN ($1, $2)`,
			userA, userB,
		)
	})

	connectedA, err := repo.HasRecentEventByUserID(context.Background(), userA, 60*time.Second)
	if err != nil {
		t.Fatalf("HasRecentEventByUserID userA: %v", err)
	}

	if !connectedA {
		t.Error("expected userA to be connected")
	}

	connectedB, err := repo.HasRecentEventByUserID(context.Background(), userB, 60*time.Second)
	if err != nil {
		t.Fatalf("HasRecentEventByUserID userB: %v", err)
	}

	if connectedB {
		t.Error("expected userB to be disconnected — must not see userA's events")
	}
}

func TestDaemonEventsRepository_Interface(t *testing.T) {
	// Compile-time check: NewDaemonEventsRepository accepts a repository.DB.
	var db repository.DB = &fakeDB{}
	repo := repository.NewDaemonEventsRepository(db)

	if repo == nil {
		t.Fatal("NewDaemonEventsRepository returned nil")
	}
}

// TestDaemonEventsRepository_Insert_SequencePersisted verifies that the sequence
// value is written to the daemon_events.sequence column (ADR-013, ticket #1521).
func TestDaemonEventsRepository_Insert_SequencePersisted(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDaemonEventsRepository(db)

	const userID int64 = 9999
	const accountID = "test-account-seq"
	const wantSequence uint64 = 42

	payload := json.RawMessage(`{"key":"value"}`)
	occurredAt := time.Now().UTC().Truncate(time.Second)

	err := repo.Insert(context.Background(), userID, accountID, "match.completed", payload, occurredAt, "evt_seq_repo_01", wantSequence)
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(
			context.Background(),
			`DELETE FROM daemon_events WHERE user_id = $1 AND account_id = $2`,
			userID, accountID,
		)
	})

	// Read back the raw sequence value to confirm it was persisted.
	var gotSequence uint64

	row := db.QueryRowContext(
		context.Background(),
		`SELECT sequence FROM daemon_events WHERE user_id = $1 AND account_id = $2`,
		userID, accountID,
	)
	if err := row.Scan(&gotSequence); err != nil {
		t.Fatalf("Scan sequence: %v", err)
	}

	if gotSequence != wantSequence {
		t.Errorf("sequence=%d, want %d", gotSequence, wantSequence)
	}
}
