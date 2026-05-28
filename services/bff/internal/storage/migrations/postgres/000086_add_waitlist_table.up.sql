-- Migration 000086: create waitlist_entries table for Phase 1 Mailchimp signup.
-- Ticket: vault-mtg-tickets#121
--
-- Design notes:
--   * email uses CITEXT so equality/uniqueness checks are case-insensitive
--     without requiring callers to lower-case before insert. Requires the
--     citext extension (CREATE EXTENSION IF NOT EXISTS citext below).
--     ADR-024: vaultmtg_app lacks superuser; Ray will enable the extension on
--     RDS before deploy if not already present. The IF NOT EXISTS guard here
--     is a safety net.
--   * mailchimp_status DEFAULT 'failed': if the process crashes between INSERT
--     and the Mailchimp API call the row is already in a reconcilable state.
--     The happy path writes 'subscribed'. A future reconciler (separate ticket)
--     picks up rows where mailchimp_status = 'failed'.
--   * referrer VARCHAR(2048): UTM-laden landing page URLs routinely exceed
--     1024 chars. Nullable — not all signups arrive via a tracked referrer.
--   * utm_source, utm_medium, utm_campaign TEXT: nullable UTM attribution fields
--     per Ray's SCHEMA EXTENSION DECISION (utm_content/utm_term deferred to v0.4).
--   * The UNIQUE constraint on email is the idempotency anchor for the
--     ON CONFLICT DO NOTHING RETURNING id upsert in the handler (RC1).

CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE waitlist_entries (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email             CITEXT      NOT NULL,
    mailchimp_status  TEXT        NOT NULL DEFAULT 'failed',
    utm_source        TEXT,
    utm_medium        TEXT,
    utm_campaign      TEXT,
    referrer          VARCHAR(2048),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT waitlist_entries_email_unique UNIQUE (email)
);

CREATE INDEX waitlist_entries_created_at_idx ON waitlist_entries (created_at DESC);
CREATE INDEX waitlist_entries_mailchimp_status_idx ON waitlist_entries (mailchimp_status) WHERE mailchimp_status = 'failed';
