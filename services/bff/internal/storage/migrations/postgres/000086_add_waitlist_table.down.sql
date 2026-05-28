-- Rollback migration 000086: drop waitlist_entries table.
--
-- The citext extension is intentionally NOT dropped here: it may be in use by
-- other tables (e.g. accounts.email), and DROP EXTENSION is destructive.
-- If citext was added solely by this migration and must be removed, do so
-- manually with coordination from the DBA.

DROP TABLE IF EXISTS waitlist_entries;
