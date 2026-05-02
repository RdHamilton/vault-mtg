package storage

// NOTE: The SQLite file-based backup tests have been removed as part of the
// PostgreSQL migration (ticket #1003). PostgreSQL backup/restore is handled
// via pg_dump/pg_restore at the infrastructure level, not via file copy.
// New backup integration tests will be added in a follow-up ticket.
