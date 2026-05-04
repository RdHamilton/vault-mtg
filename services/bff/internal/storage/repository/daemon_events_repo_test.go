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

	err := repo.Insert(context.Background(), 1, "test-account-1", "match.game_started", payload, occurredAt)
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

func TestDaemonEventsRepository_ListByUserID_OrderedNewestFirst(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDaemonEventsRepository(db)

	const userID int64 = 9991
	const accountID = "test-account-ordered"

	older := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	newer := time.Now().UTC().Add(-1 * time.Hour).Truncate(time.Second)

	payload := json.RawMessage(`{"seq":1}`)

	if err := repo.Insert(context.Background(), userID, accountID, "event.a", payload, older); err != nil {
		t.Fatalf("Insert older: %v", err)
	}

	payload2 := json.RawMessage(`{"seq":2}`)

	if err := repo.Insert(context.Background(), userID, accountID, "event.b", payload2, newer); err != nil {
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

	if err := repo.Insert(context.Background(), userA, accountA, "event.x", payload, occurredAt); err != nil {
		t.Fatalf("Insert userA: %v", err)
	}

	if err := repo.Insert(context.Background(), userB, accountB, "event.y", payload, occurredAt); err != nil {
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

func TestDaemonEventsRepository_Interface(t *testing.T) {
	// Compile-time check: NewDaemonEventsRepository accepts a repository.DB.
	var db repository.DB = &fakeDB{}
	repo := repository.NewDaemonEventsRepository(db)

	if repo == nil {
		t.Fatal("NewDaemonEventsRepository returned nil")
	}
}
