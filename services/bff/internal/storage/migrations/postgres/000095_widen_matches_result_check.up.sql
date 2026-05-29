-- Migration 000095: widen matches.result CHECK constraint to include 'unknown'.
--
-- The projection worker now defaults result to 'unknown' when the daemon emits
-- a match.completed event with an indeterminate result (both winning_team_id
-- and player_team_id are zero and no pre-computed result string is present).
-- Without this migration, an 'unknown' result value would violate the existing
-- CHECK(result IN ('win', 'loss')) constraint and route the event to the DLQ,
-- defeating the graceful-degradation fix in vault-mtg-tickets#200.
--
-- The auto-generated constraint name for an unnamed inline CHECK on the
-- matches table is 'matches_result_check' (PostgreSQL convention:
-- <table>_<column>_check).  Verified against the live schema definition in
-- migration 000054.
--
-- Analytics queries use FILTER (WHERE lower(result) = 'win') and
-- FILTER (WHERE lower(result) = 'loss') — 'unknown' rows are simply excluded
-- from win/loss aggregates, which is the correct behavior.

ALTER TABLE matches
    DROP CONSTRAINT matches_result_check,
    ADD CONSTRAINT matches_result_check
        CHECK (result IN ('win', 'loss', 'unknown'));
