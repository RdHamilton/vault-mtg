-- Migration 000089: reset the AED skip-guard counter in sync_hashes.
-- Ticket: vault-mtg-tickets#164
--
-- The sync Lambda accumulated a skip-guard count of 7 for set AED because
-- the updateSkipGuard function previously returned a fatal error when the
-- counter reached the threshold (maxConsecutiveSkips=3), causing EventBridge
-- to treat the invocation as failed. The counter was never reset because each
-- subsequent invocation tripped the guard before reaching the reset path.
--
-- Migration 000088 set seventeenlands_code='DFT' so 17Lands API calls for AED
-- now use the correct expansion code. With the code fix in this wave (updateSkipGuard
-- is now non-fatal), the counter will reset naturally on the next successful
-- 17Lands response. This migration resets it immediately so the guard starts
-- from a clean state rather than counting from 7.

DELETE FROM sync_hashes
WHERE  key = 'skip_count:AED';
