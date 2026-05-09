# VaultMTG NPS Survey — Q2 2026

**Form ID**: `QV2fMEfN`
**Public Link**: https://form.typeform.com/to/QV2fMEfN
**API Responses Endpoint**: https://api.typeform.com/forms/QV2fMEfN/responses

## Survey Questions

1. **NPS Question** (required)
   - "How likely are you to recommend VaultMTG to a friend who plays MTG Arena?"
   - Type: NPS (0-10 scale)
   - Field ID: `zCmVQfF3WDVS`
   - Ref: `01KR4GNNFRW6W8ED9MCFAXYZK4`

2. **Open Text** (optional)
   - "What's the one thing that would make VaultMTG better for you?"
   - Type: Long text
   - Field ID: `w4U7DfttneHF`
   - Ref: `01KR4GNNFRBCMRP3BVQH9E7D7Q`

## Deployment Notes

- Created: 2026-05-08
- Public: Yes (enable users to access via link)
- Progress bar: Enabled
- Typeform branding: Enabled
- Autosave: Enabled

## To collect responses

Use the Typeform API token from SSM (`/vaultmtg/prod/typeform-api-token`) to fetch responses:
```bash
curl -H "Authorization: Bearer $TYPEFORM_TOKEN" \
  https://api.typeform.com/forms/QV2fMEfN/responses
```

## Distribution

- Post link in Discord `#announcements` (coordinate with growth-marketing)
- Add to in-app feedback banner or modal
- Link in email campaign (coordinate with growth-marketing)
