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

### Step 3: Capture the Baseline

After adding the token, the next push to `main` that touches `frontend/src/**` or `.storybook/**` will trigger the Chromatic workflow. Chromatic will upload the Storybook build and create the initial baseline. All stories will be auto-accepted on the first run.

### Step 4: Document the Project URL

Once the project is created on Chromatic, update this file with the actual project URL:

**Chromatic Project URL**: `TODO — update once project is created at chromatic.com`

Expected format: `https://www.chromatic.com/builds?appId=<app-id>`

---

## CI Behavior

The Chromatic workflow (`.github/workflows/chromatic.yml`) behaves as follows:

- **Token not set**: CI step is skipped with a warning. The rest of CI still passes. This allows unblocked devs to merge until the token is provisioned.
- **Token set, no visual changes**: Chromatic passes, CI green.
- **Token set, unreviewed visual changes detected**: Chromatic exits with a non-zero code, CI fails. A human must review and approve/reject changes on the Chromatic dashboard before the PR can merge.

This makes Chromatic a real visual regression gate, not a pass-through.
