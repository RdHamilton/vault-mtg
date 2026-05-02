package repository

// NOTE: The SQLite-based EDHREC repository tests have been removed as part of the
// PostgreSQL migration (ticket #1003). PostgreSQL integration tests require
// a live DATABASE_URL and will be added in a follow-up ticket.
//
// The GetMatchingThemes function (pure logic, no DB) can be unit-tested independently.
