-- Migration 000087: disable AED from draft-active sync and reset skip guard.
-- Ticket: vault-mtg-tickets#47
--
-- Root cause: the sync Lambda queries 17Lands using the Scryfall set code
-- verbatim. 17Lands returns an empty array for "AED" (Aetherdrift), which is
-- not their expansion code for the set. Three consecutive empty responses
-- tripped the circuit-breaker (maxConsecutiveSkips=3) and fired an alarm.
--
-- This hotfix removes AED from the draft-active sync loop immediately by
-- setting is_draft_active = FALSE. The permanent fix (adding a
-- seventeenlands_code column and setting it to 'DFT' for AED) ships in PR2
-- (migration 000088) after ticket #46 merges, since that PR overlaps the same
-- lambda.go code path.
--
-- The skip-guard row is deleted here (not in a separate manual Lambda
-- invocation) so the reset is captured in version control and replays
-- correctly on a fresh environment.

UPDATE sets
SET    is_draft_active = FALSE
WHERE  code = 'AED';

-- Clear the circuit-breaker counter so re-enabling AED in migration 000088
-- starts from a clean state rather than the already-tripped value of 3.
DELETE FROM sync_hashes
WHERE  key = 'skip_count:AED';
