-- Revert from_draft_pick column type: BOOLEAN → INTEGER (PostgreSQL)

ALTER TABLE deck_cards ALTER COLUMN from_draft_pick DROP DEFAULT;
ALTER TABLE deck_cards ALTER COLUMN from_draft_pick TYPE INTEGER USING (from_draft_pick::integer);
ALTER TABLE deck_cards ALTER COLUMN from_draft_pick SET DEFAULT 0;
ALTER TABLE deck_cards ADD CONSTRAINT deck_cards_from_draft_pick_check CHECK (from_draft_pick IN (0, 1));
