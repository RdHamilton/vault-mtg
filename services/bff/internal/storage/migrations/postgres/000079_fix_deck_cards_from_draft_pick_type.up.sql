ALTER TABLE deck_cards DROP CONSTRAINT IF EXISTS deck_cards_from_draft_pick_check;
ALTER TABLE deck_cards ALTER COLUMN from_draft_pick DROP DEFAULT;
ALTER TABLE deck_cards ALTER COLUMN from_draft_pick TYPE BOOLEAN USING (from_draft_pick::boolean);
ALTER TABLE deck_cards ALTER COLUMN from_draft_pick SET DEFAULT false;
