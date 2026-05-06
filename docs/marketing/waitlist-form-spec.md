# Waitlist Form — Front-Engineer Spec

Ticket: #1409
Author: growth-marketing
Date: 2026-05-06

---

## Mailchimp Credentials (SSM)

| Parameter | SSM Path |
|---|---|
| API key | `/vaultmtg/prod/mailchimp-api-key` |
| List (audience) ID | `/vaultmtg/prod/mailchimp-list-id` |

Both parameters are already stored in SSM. Retrieve them at runtime via the BFF — do NOT expose either value in the frontend bundle or any environment variable shipped to the browser.

---

## BFF Endpoint

Create a new public route (no auth required — waitlist is pre-login):

```
POST /api/v1/waitlist
```

### Request body

```json
{
  "email": "user@example.com",
  "first_name": "Jane"
}
```

| Field | Type | Required | Notes |
|---|---|---|---|
| `email` | string | yes | standard email validation |
| `first_name` | string | no | used as Mailchimp `FNAME` merge field |

### Success response

```json
{ "status": "ok" }
```

HTTP 200.

### Error responses

- `400` — missing or invalid email
- `409` — email already subscribed (Mailchimp returns 400 with title "Member Exists" — surface as 409 to the frontend so the UI can show a friendly "you're already on the list" message)
- `502` — Mailchimp API error (log server-side, return generic error to client)

---

## Mailchimp API Call (server-side)

The BFF calls the Mailchimp Members API to add the subscriber:

```
POST https://<dc>.api.mailchimp.com/3.0/lists/<LIST_ID>/members
```

Where `<dc>` is the data center extracted from the API key (the suffix after the last `-`, e.g. `us21`).

### Request body to Mailchimp

```json
{
  "email_address": "<email>",
  "status": "subscribed",
  "merge_fields": {
    "FNAME": "<first_name>"
  },
  "tags": ["waitlist"]
}
```

The `tags` array applies the `waitlist` tag to every signup. This tag is already created in the Mailchimp audience.

### Authentication

HTTP Basic auth: username = `"anystring"` (Mailchimp ignores it), password = API key from SSM.

---

## Frontend Behavior

### Form fields

```html
<input type="text"  name="first_name" placeholder="First name (optional)" />
<input type="email" name="email"       placeholder="Email address"         required />
<button type="submit">Join the Waitlist</button>
```

Use the label and button text from `docs/marketing/content/2026-05-waitlist-copy.md`. The copy file has separate versions for `/waitlist` and the `/download` embedded section.

### Success state

Hide the form, show the confirmation message from the copy doc:

> You are on the list. We will email you at [email] when the VaultMTG beta opens.

### Already-subscribed state (409)

Show inline, do not hide the form:

> You're already on the list — we'll reach out when beta opens.

### Error state (non-409 failure)

> Something went wrong. Try again or email us at hello@vaultmtg.app.

---

## Analytics

Fire a PostHog event on successful submission (HTTP 200 from BFF):

```js
posthog.capture('waitlist_signup', { source: window.location.pathname })
```

The `source` property distinguishes `/waitlist` signups from `/download` embedded-form signups.

---

## Mailchimp Automation

A welcome/confirmation email is triggered automatically by Mailchimp on subscription. The BFF does not send any email directly — Mailchimp handles it via the audience automation set up in the dashboard.

- Subject: "You're on the VaultMTG waitlist"
- Triggered: on `status: subscribed`

---

## Mobile Responsiveness

The form must be usable on mobile. Single-column layout, full-width inputs, touch-friendly button (min 44px height). No additional requirements beyond standard responsive design.
