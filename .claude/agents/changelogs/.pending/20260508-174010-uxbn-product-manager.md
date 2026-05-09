target: product-manager
---
## 2026-05-08 — Smart Craft Next ML feature research + PRD
**Triggered by**: User request — find best free-tier ML opportunity for v0.4.0 targeting 50K MAU
**Decision**: Recommended "Smart Craft Next" (personalized cards-to-craft ranking) over draft pick grades, post-match analysis, and opponent prediction. Draft grades are table stakes (17lands open-source tool already covers them). Smart Craft Next is the only feature that closes the match-data feedback loop with the user's own win history — a differentiation none of the competitors have in a free tier. Implementation is zero per-request inference (nightly Lambda batch + static table read). RICE score: 6,000.
**Output**: docs/prd/0005-smart-craft-next.md
**RICE score**: Reach 8K, Impact 3, Confidence 75%, Effort 3pw → Score 6,000
