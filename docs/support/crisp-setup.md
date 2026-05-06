# Crisp Live Chat — Setup Guide

## What Is Crisp?

Crisp is the live chat provider for VaultMTG. It adds a chat widget to vaultmtg.app so users can contact support directly from the app or marketing site without leaving their browser. The free tier supports unlimited conversations and is sufficient for beta.

---

## Who Does What

| Step | Owner |
|---|---|
| Create Crisp account | Ray (manual step) |
| Get the website ID | Ray (from Crisp dashboard) |
| Install widget script on vaultmtg.app | front-engineer |
| Monitor the Crisp inbox | customer-success |

---

## Step 1: Create a Crisp Account (Ray)

1. Go to https://crisp.chat
2. Click "Start for Free" and sign up with your work email
3. Create a new website called "VaultMTG"
4. From the Crisp dashboard: Settings > Website Settings > Website ID
5. Copy the website ID (it looks like: `12345678-abcd-1234-abcd-1234567890ab`)
6. Share the website ID with the front-engineer

---

## Step 2: Install the Widget Script (front-engineer)

Add the following script to the `<head>` of the vaultmtg.app marketing site. Replace `YOUR_WEBSITE_ID` with the actual website ID from the Crisp dashboard.

```html
<script type="text/javascript">
  window.$crisp = [];
  window.CRISP_WEBSITE_ID = "YOUR_WEBSITE_ID";
  (function () {
    var d = document;
    var s = d.createElement("script");
    s.src = "https://client.crisp.chat/l.js";
    s.async = 1;
    d.getElementsByTagName("head")[0].appendChild(s);
  })();
</script>
```

This script is provided by Crisp and is safe to place in `<head>`. It loads asynchronously and does not block page rendering.

### Implementation Notes

- Place the script in the marketing site's base HTML template so it appears on every page
- Do not place it only on specific pages — users may need help from any page
- The widget will appear as a chat bubble in the bottom-right corner of the site
- No backend changes are required; Crisp handles message routing entirely

---

## Step 3: Verify the Widget Is Live

After deploying the script:
1. Open vaultmtg.app in a browser
2. Confirm the Crisp chat bubble appears in the bottom-right corner
3. Send a test message
4. Verify the message appears in the Crisp inbox at app.crisp.chat

---

## Crisp Inbox Configuration (customer-success)

After the account is created, configure the inbox:

1. Set business hours in Crisp: Settings > Availability
2. Set an away message for outside business hours: "Thanks for reaching out — we'll get back to you within 24 hours."
3. Add teammate(s) in Settings > Teammates
4. Create saved replies for the most common questions (FAQ #1 through #5)

---

## Privacy / Data

Crisp collects the user's browser, OS, and page URL on chat start. This is standard for live chat tools. Update the vaultmtg.app privacy policy to mention Crisp before beta launch.

---

## Upgrade Path

The Crisp free tier supports all beta needs. If the inbox volume grows beyond what the free tier handles, the next tier is Crisp Pro ($25/month). Do not upgrade without discussing with Ray first.
