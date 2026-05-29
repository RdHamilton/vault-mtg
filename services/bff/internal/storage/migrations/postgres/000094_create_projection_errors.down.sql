-- Rollback migration 000094: drop projection_errors DLQ table.
DROP TABLE IF EXISTS projection_errors;
