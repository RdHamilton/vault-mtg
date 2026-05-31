# Storybook + Chromatic Spike — React 19 + Vite 7

**Date**: 2026-05-10
**Ticket**: #1621
**Verdict**: GO

## Summary

Storybook 8 was the original target but does **not** support Vite 7 (`peer vite@"^4.0.0 || ^5.0.0 || ^6.0.0"`). The project uses Vite 7.x. Using Storybook 8 with Vite 7 would require `--legacy-peer-deps` and produce an unsupported configuration.

**Resolution**: Use **Storybook 10** (latest stable as of 2026-05-10), which explicitly supports Vite `^5.0.0 || ^6.0.0 || ^7.0.0 || ^8.0.0` and React `^16.8.0 || ^17.0.0 || ^18.0.0 || ^19.0.0`. This is a first-class supported configuration.

## Versions Chosen

| Package | Version |
|---|---|
| `storybook` | 10.3.6 |
| `@storybook/react` | 10.3.6 |
| `@storybook/react-vite` | 10.3.6 |
| `chromatic` | 16.9.1 |

## Framework Config

- Framework: `@storybook/react-vite` — Storybook's Vite-native builder. Uses the project's existing `vite.config.ts` by default.
- No webpack. No babel. Full Vite HMR in Storybook dev server.

## Gotchas / Findings

1. **Storybook 8 incompatible with Vite 7** — peer dep constraint is `vite@"^4–6"`. Do not attempt to use Storybook 8 on this project.
2. **React 19 compat** — Storybook 10 supports React 19 natively. No special configuration needed.
3. **`@storybook/addon-essentials` ships as part of Storybook 10 core** — separate addon installation not required for basic docs/controls/viewport/actions.
4. **Chromatic token** — `CHROMATIC_PROJECT_TOKEN` secret must be added to GitHub Actions by Ray before the Chromatic CI step will run. See provisioning steps below.
5. **Existing test suite unaffected** — Vitest and Playwright configs are unchanged. Story files (`*.stories.tsx`) are not picked up by Vitest (`exclude` list covers dist/e2e; stories do not match test globs).

## Decision

GO. Ship Storybook 10 + `@storybook/react-vite` as the component library foundation. Wire Chromatic as a required CI status once the token is provisioned.

---

## Chromatic Setup — Manual Steps for Ray

### Step 1: Create a Chromatic Project

1. Go to [https://www.chromatic.com](https://www.chromatic.com) and sign in (use your GitHub account — `RdHamilton`).
2. Click **Add project** and select the `RdHamilton/MTGA-Companion` repository.
3. Chromatic will display your **Project Token** (format: `chpt_xxxxxxxxxxxx`).
4. Copy the token — you will need it in Step 2.

### Step 2: Add the Token to GitHub Actions

1. Go to [https://github.com/RdHamilton/MTGA-Companion/settings/secrets/actions](https://github.com/RdHamilton/MTGA-Companion/settings/secrets/actions).
2. Click **New repository secret**.
3. Name: `CHROMATIC_PROJECT_TOKEN`
4. Value: paste the token copied from Step 1.
5. Click **Add secret**.

### Step 3: Approve the Initial Baseline

Steps 1–2 are already complete (project created, token provisioned as of 2026-05-10). The first Chromatic run found 23 unreviewed visual changes — these are all new stories, not regressions. Go to the build and accept all to establish the baseline:

**Build to review**: `https://www.chromatic.com/build?appId=6a011ad4409d5fcfc80e5a25&number=4`

Click **Accept all** on this build. Subsequent runs will only flag genuine visual regressions.

### Project URL (provisioned 2026-05-10)

**Chromatic Project URL**: `https://www.chromatic.com/builds?appId=6a011ad4409d5fcfc80e5a25`

App ID: `6a011ad4409d5fcfc80e5a25`

---

## CI Behavior

The Chromatic workflow (`.github/workflows/chromatic.yml`) behaves as follows:

- **Token not set**: CI step is skipped with a warning. The rest of CI still passes. This allows unblocked devs to merge until the token is provisioned.
- **Token set, no visual changes**: Chromatic passes, CI green.
- **Token set, unreviewed visual changes detected**: Chromatic exits with a non-zero code, CI fails. A human must review and approve/reject changes on the Chromatic dashboard before the PR can merge.

This makes Chromatic a real visual regression gate, not a pass-through.

## `brand-approved` Auto-Accept (Epic B brand PRs)

Epic B brand PRs apply the frozen design-system tokens (`frontend/src/index.css`
sourced from "Ray Hamilton Engineering Design System/colors_and_type.css"). These
produce **intentional** Chromatic visual diffs across nearly every story, which
would otherwise require accepting each one on the Chromatic dashboard.

To keep these PRs out of the per-PR dashboard-accept loop, the workflow supports a
**`brand-approved`** label:

- When a PR carries the `brand-approved` label, the `Run Chromatic` step adds
  `--auto-accept-changes`, so Chromatic accepts **all** detected visual changes for
  that build, exits 0, and the required `Chromatic Visual Tests` check goes **green**
  — without anyone opening the Chromatic dashboard.
- Applying or removing the label re-runs the workflow (the `pull_request` trigger
  includes the `labeled`/`unlabeled` types), so the check re-evaluates as soon as a
  reviewer labels the PR.
- A PR **without** the label is unchanged: detected visual changes keep the check red
  until they are accepted on the dashboard.
- `main` pushes are unaffected — the auto-accept-on-push baseline path is selected on
  `github.event_name == 'push'` only, never on a label.

### Label contract — the safety is code review, not visual inspection

`--auto-accept-changes` accepts **every** visual change in the build with no
per-snapshot inspection. The `brand-approved` label therefore means, exactly:

> A reviewer has verified that **every** visual change in this PR is an intentional,
> design-system-matching brand change — by diffing the PR's CSS against the frozen
> `colors_and_type.css` — and accepts all of them sight-unseen on the dashboard.

**Never apply `brand-approved` blind.** Only a reviewer who has completed the CSS-vs-
design-system review applies it. On Epic B, that reviewer is Lee. Once labeled, the
re-run clears the Chromatic gate and the PR can merge.
