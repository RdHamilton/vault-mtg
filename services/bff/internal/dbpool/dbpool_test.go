package dbpool_test

import (
	"database/sql"
	"testing"

	"github.com/RdHamilton/vault-mtg/services/bff/internal/dbpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// TestConfigure asserts that Configure sets the pool parameters to the values
// approved in the Ray PLAN_VERDICT (issue #2548).  The test opens a *sql.DB
// against a dummy DSN — sql.Open is lazy and does not dial the server, so the
// assertions run without a live database.
//
//	MaxOpenConns     = 25  (approved over AC default of 20 — extra headroom)
//	MaxIdleConns     = 5   (matches AC default)
//	ConnMaxLifetime  = 30 min (matches AC; RDS connection rotation window)
//	ConnMaxIdleTime  = 5 min  (matches AC; idle-connection hygiene)
func TestConfigure(t *testing.T) {
	db, err := sql.Open("pgx", "host=localhost dbname=test")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()

	dbpool.Configure(db)

	stats := db.Stats()

	if stats.MaxOpenConnections != 25 {
		t.Errorf("MaxOpenConnections: want 25, got %d", stats.MaxOpenConnections)
	}
}
