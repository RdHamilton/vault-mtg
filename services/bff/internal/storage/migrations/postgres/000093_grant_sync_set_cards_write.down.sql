-- Revoke the INSERT and UPDATE grants on set_cards added in 000093.
REVOKE INSERT, UPDATE ON set_cards FROM mtga_sync;
