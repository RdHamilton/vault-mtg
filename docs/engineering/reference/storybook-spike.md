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
4. **Chromatic token** — `CHROMATIC_PROJECT_TOKEN` secret must be added to GitHub Actions by Ray before the Chromatic CI step will run. See `REQUIRES` comment in `.github/workflows/chromatic.yml`.
5. **Existing test suite unaffected** — Vitest and Playwright configs are unchanged. Story files (`*.stories.tsx`) are not picked up by Vitest (`exclude` list covers dist/e2e; stories do not match test globs).

## Decision

GO. Ship Storybook 10 + `@storybook/react-vite` as the component library foundation. Wire Chromatic as a required CI status once the token is provisioned.
