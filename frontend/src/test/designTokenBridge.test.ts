/**
 * Design-token bridge guard test — updated for #339 Pass C.
 *
 * Pass A (#309) installed the TEMPORARY legacy-alias bridge so ~600 component
 * references to undefined legacy names resolved to the sapphire palette.
 * Pass C (#339) migrated every call-site to the canonical semantic tokens
 * (--fg/--bg/--accent/--border/…) and deleted the bridge block from index.css.
 *
 * This test now enforces the post-bridge contract:
 *   1. The canonical semantic layer tokens are defined in index.css.
 *   2. No legacy alias names remain in component CSS (the bridge is gone;
 *      any stray reference would silently fall back to its hex default).
 *   3. Every var()-valued definition in index.css points at a defined property
 *      (no dangling pointers — catches regressions if a primitive is renamed).
 */
import { describe, it, expect } from 'vitest';
import { readFileSync, readdirSync, statSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const SRC_DIR = join(dirname(fileURLToPath(import.meta.url)), '..');
const INDEX_CSS = join(SRC_DIR, 'index.css');

/**
 * Genuinely categorical / data-viz accents that need distinct hues to stay
 * legible. Per Ray's APPROVE-WITH-CHANGES (#309) ruling #2 (Option B), these
 * are intentionally NOT mapped to the brand palette. Listed explicitly so the
 * omission is deliberate, not accidental.
 */
const CATEGORICAL_ALLOWLIST = new Set([
  '--bar-bg',
  '--ml-bg',
  '--ml-color',
  '--meta-bg',
  '--meta-color',
  '--personal-bg',
  '--personal-color',
]);

/**
 * Legacy alias names that lived in the #309 bridge block. After #339 these must
 * NOT appear in any component CSS file — they are undefined and would silently
 * fall back to whatever hex default the var() author wrote.
 */
const LEGACY_ALIAS_NAMES = new Set([
  '--text-primary',
  '--color-text-primary',
  '--color-text',
  '--text-secondary',
  '--color-text-secondary',
  '--text-muted',
  '--text-tertiary',
  '--color-text-muted',
  '--bg-primary',
  '--background-color',
  '--bg-secondary',
  '--card-bg',
  '--card-bg-dark',
  '--surface-color',
  '--color-surface',
  '--bg-tertiary',
  '--color-surface-elevated',
  '--bg-hover',
  '--bg-sunken',
  '--border-color',
  '--color-border',
  '--border-hover',
  '--focus-ring',
  '--accent-color',
  '--color-accent',
  '--color-primary',
  '--accent-rgb',
  '--secondary-bg',
  '--secondary-hover',
  '--error-color',
  '--color-error',
  '--loss-color',
  '--error-bg',
  '--success-color',
  '--color-success',
  '--win-color',
  '--success-bg',
  '--warning-color',
  '--info-color',
  '--info-bg',
  '--tier-1-color',
  '--tier-2-color',
  '--tier-3-color',
  '--tier-4-color',
]);

function listCssFiles(dir: string): string[] {
  const out: string[] = [];
  for (const entry of readdirSync(dir)) {
    if (entry === 'node_modules' || entry === 'dist') continue;
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) {
      out.push(...listCssFiles(full));
    } else if (entry.endsWith('.css')) {
      out.push(full);
    }
  }
  return out;
}

/** All custom properties REFERENCED via var(--name[, fallback]) in a file. */
function referencedVars(css: string): Set<string> {
  const names = new Set<string>();
  const re = /var\(\s*(--[a-zA-Z0-9-]+)/g;
  let m: RegExpExecArray | null;
  while ((m = re.exec(css)) !== null) names.add(m[1]);
  return names;
}

/** All custom properties DEFINED (`--name: …;`) in a file. */
function definedVars(css: string): Set<string> {
  const names = new Set<string>();
  const re = /^\s*(--[a-zA-Z0-9-]+)\s*:/gm;
  let m: RegExpExecArray | null;
  while ((m = re.exec(css)) !== null) names.add(m[1]);
  return names;
}

describe('#339 — bridge removed, canonical tokens only', () => {
  const indexCss = readFileSync(INDEX_CSS, 'utf8');
  const defined = definedVars(indexCss);

  it('defines the canonical semantic layer mapped onto --vault-* primitives', () => {
    for (const token of [
      '--fg',
      '--fg-secondary',
      '--fg-muted',
      '--bg',
      '--bg-raised',
      '--bg-overlay',
      '--border',
      '--accent',
      '--accent-hover',
      '--accent-press',
      '--accent-dim',
      '--accent-dim-hover',
      '--success',
      '--danger',
      '--warning',
      '--info',
    ]) {
      expect(defined.has(token), `semantic token ${token} must be defined in index.css`).toBe(true);
    }
  });

  it('legacy-alias bridge block is gone from index.css', () => {
    const legacyStillDefined = [...LEGACY_ALIAS_NAMES].filter((name) => defined.has(name));
    expect(
      legacyStillDefined,
      `legacy alias(es) still defined in index.css (bridge not fully removed): ${legacyStillDefined.join(', ')}`,
    ).toEqual([]);
  });

  it('no component CSS references legacy alias names (bridge is gone — stray references silently fall back to hex)', () => {
    const componentFiles = listCssFiles(SRC_DIR).filter((f) => f !== INDEX_CSS);
    const offenders: string[] = [];
    for (const f of componentFiles) {
      const refs = referencedVars(readFileSync(f, 'utf8'));
      const stray = [...refs].filter((name) => LEGACY_ALIAS_NAMES.has(name));
      if (stray.length) offenders.push(`${f}: ${stray.join(', ')}`);
    }
    expect(
      offenders,
      `component CSS still references legacy aliases (migrate to canonical --fg/--bg/--accent/--border tokens): ${offenders.join('; ')}`,
    ).toEqual([]);
  });

  it('all component CSS var() references resolve (except categorical accents)', () => {
    const componentFiles = listCssFiles(SRC_DIR).filter((f) => f !== INDEX_CSS);
    const referenced = new Set<string>();
    for (const f of componentFiles) {
      for (const name of referencedVars(readFileSync(f, 'utf8'))) referenced.add(name);
    }

    const unresolved = [...referenced].filter(
      (name) => !defined.has(name) && !CATEGORICAL_ALLOWLIST.has(name),
    );

    expect(
      unresolved,
      `custom properties referenced by component CSS but not defined in index.css ` +
        `(would render via hardcoded hex fallback): ${unresolved.join(', ')}`,
    ).toEqual([]);
  });

  it('has no dangling pointer in index.css (every var()-valued definition targets a defined property)', () => {
    const re = /^\s*(--[a-zA-Z0-9-]+)\s*:\s*var\(\s*(--[a-zA-Z0-9-]+)/gm;
    const dangling: string[] = [];
    let m: RegExpExecArray | null;
    while ((m = re.exec(indexCss)) !== null) {
      const [, name, target] = m;
      if (!defined.has(target)) dangling.push(`${name} -> ${target}`);
    }
    expect(dangling, `index.css pointers with undefined targets: ${dangling.join(', ')}`).toEqual([]);
  });
});
