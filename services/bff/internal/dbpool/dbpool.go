// Package dbpool centralises connection-pool tuning for the BFF's *sql.DB
// instance.  All values were reviewed and approved in the Ray PLAN_VERDICT for
// issue #2548.
//
// Pool sizing rationale (db.t3.micro, max_connections ≈ 107):
//
//	MaxOpenConns = 25  — accepted over the AC default of 20; gives extra
//	                     headroom while leaving room for migrations, monitoring,
//	                     and the sync Lambda.
//	MaxIdleConns = 5   — keep a small warm pool; avoids cold-connect latency
//	                     on bursty workloads without holding idle sockets.
//	ConnMaxLifetime = 30 min — intentionally shorter than the 90-day RDS master-
//	                     secret rotation window so connections roll over well
//	                     within the credential window.
//	ConnMaxIdleTime = 5 min — reclaim idle connections promptly to stay below
//	                     the MaxOpenConns ceiling during low-traffic windows.
package dbpool

import (
	"database/sql"
	"time"
)

// Configure applies the approved connection-pool parameters to db.  It must be
// called once, immediately after sql.Open, before any queries are issued.
func Configure(db *sql.DB) {
	// Max 25 open connections to db.t3.micro (max_connections ≈ 107).
	db.SetMaxOpenConns(25)
	// Keep at most 5 idle connections in the pool.
	db.SetMaxIdleConns(5)
	// Roll over connections every 30 minutes — well within the 90-day RDS
	// master-secret rotation window.
	db.SetConnMaxLifetime(30 * time.Minute)
	// Release connections that have been idle for 5 minutes.
	db.SetConnMaxIdleTime(5 * time.Minute)
}
