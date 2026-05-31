/**
 * WCAG AA contrast guard — #320.
 *
 * Audits the key foreground/background token pairs used across the refreshed
 * SPA views and asserts each meets its WCAG AA threshold:
 *   - ≥4.5:1 for normal text (< 24px regular or < 19px bold)
 *   - ≥3.0:1  for large text (≥24px regular or ≥19px bold) and UI elements
 *
 * Hex values are sourced from index.css primitives. When index.css changes a
 * primitive, this test catches regressions before they ship.
 *
 * Companion to:
 *   - noRawHex.test.ts        — no raw hex outside index.css + categorical allowlist
 *   - buttonCardTokens.test.ts — button/card token compliance
 *   - designTokenBridge.test.ts — every var() reference resolves
 */
import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const SRC_DIR = join(dirname(fileURLToPath(import.meta.url)), '..');

// ---------------------------------------------------------------------------
// WCAG 2.1 contrast math
// ---------------------------------------------------------------------------

function sRGBtoLinear(c: number): number {
  const n = c / 255;
  return n <= 0.03928 ? n / 12.92 : Math.pow((n + 0.055) / 1.055, 2.4);
}

function luminance(r: number, g: number, b: number): number {
  return 0.2126 * sRGBtoLinear(r) + 0.7152 * sRGBtoLinear(g) + 0.0722 * sRGBtoLinear(b);
}

function contrastRatio(hex1: string, hex2: string): number {
  const parse = (h: string): [number, number, number] => {
    h = h.replace('#', '');
    if (h.length === 3) h = h.split('').map((c) => c + c).join('');
    return [parseInt(h.slice(0, 2), 16), parseInt(h.slice(2, 4), 16), parseInt(h.slice(4, 6), 16)];
  };
  const [r1, g1, b1] = parse(hex1);
  const [r2, g2, b2] = parse(hex2);
  const l1 = luminance(r1, g1, b1);
  const l2 = luminance(r2, g2, b2);
  const lighter = Math.max(l1, l2);
  const darker = Math.min(l1, l2);
  return (lighter + 0.05) / (darker + 0.05);
}

// ---------------------------------------------------------------------------
// Token values — extracted from index.css primitives.
// If index.css changes these, the tests below will catch it.
// ---------------------------------------------------------------------------

/**
 * Parse --token-name: #hexvalue; entries from index.css.
 * Returns a map of token name → hex string.
 */
function parseTokens(css: string): Map<string, string> {
  const map = new Map<string, string>();
  const re = /(--vault-[a-zA-Z0-9-]+)\s*:\s*(#[0-9a-fA-F]{3,8})/g;
  let m: RegExpExecArray | null;
  while ((m = re.exec(css)) !== null) {
    map.set(m[1], m[2]);
  }
  return map;
}

const indexCss = readFileSync(join(SRC_DIR, 'index.css'), 'utf8');
const tokens = parseTokens(indexCss);

function tok(name: string): string {
  const v = tokens.get(name);
  if (!v) throw new Error(`Token ${name} not found in index.css`);
  return v;
}

// ---------------------------------------------------------------------------
// Contrast pairs — (label, fg token, bg token, threshold)
// ---------------------------------------------------------------------------

// Normal-text pairs — must meet 4.5:1 (WCAG AA)
describe('#320 WCAG AA — normal-text contrast (≥4.5:1)', () => {
  const pairs: Array<[string, string, string]> = [
    // Primary text on all three surface levels
    ['--vault-fg on --vault-bg (page base)',         '--vault-fg',           '--vault-bg'],
    ['--vault-fg on --vault-bg-raised (cards)',       '--vault-fg',           '--vault-bg-raised'],
    ['--vault-fg on --vault-bg-overlay (modals)',     '--vault-fg',           '--vault-bg-overlay'],
    // Secondary text
    ['--vault-fg-secondary on --vault-bg',           '--vault-fg-secondary', '--vault-bg'],
    ['--vault-fg-secondary on --vault-bg-raised',    '--vault-fg-secondary', '--vault-bg-raised'],
    ['--vault-fg-secondary on --vault-bg-overlay',   '--vault-fg-secondary', '--vault-bg-overlay'],
    // Muted text — the #320 regression fix (was #4E6080 = 2.98:1; now #7890AA)
    ['--vault-fg-muted on --vault-bg',               '--vault-fg-muted',     '--vault-bg'],
    ['--vault-fg-muted on --vault-bg-raised',        '--vault-fg-muted',     '--vault-bg-raised'],
    ['--vault-fg-muted on --vault-bg-overlay',       '--vault-fg-muted',     '--vault-bg-overlay'],
    // Inverse text (buttons)
    ['--vault-fg-inverse on --vault-sapphire (CTA)', '--vault-fg-inverse',   '--vault-sapphire'],
  ];

  for (const [label, fgToken, bgToken] of pairs) {
    it(label, () => {
      const r = contrastRatio(tok(fgToken), tok(bgToken));
      expect(
        r,
        `${label}: ${r.toFixed(2)}:1 < 4.5:1 (WCAG AA normal text). ` +
          `fg=${tok(fgToken)} bg=${tok(bgToken)}`,
      ).toBeGreaterThanOrEqual(4.5);
    });
  }
});

// UI-element / large-text pairs — must meet 3.0:1 (WCAG AA)
describe('#320 WCAG AA — UI element / large-text contrast (≥3.0:1)', () => {
  const pairs: Array<[string, string, string]> = [
    // Focus ring: --vault-sapphire-dark outline on dark surfaces
    ['--vault-sapphire-dark focus ring on --vault-bg-raised', '--vault-sapphire-dark', '--vault-bg-raised'],
    ['--vault-sapphire-dark focus ring on --vault-bg',        '--vault-sapphire-dark', '--vault-bg'],
    // Sapphire accent used for active nav indicator (border-bottom) on bg-raised
    ['--vault-sapphire active indicator on --vault-bg-raised', '--vault-sapphire',     '--vault-bg-raised'],
    // State colors used as large headings / icon fills
    ['--vault-success on --vault-bg-raised',                  '--vault-success',       '--vault-bg-raised'],
    ['--vault-danger on --vault-bg-raised',                   '--vault-danger',        '--vault-bg-raised'],
    ['--vault-warning on --vault-bg-raised',                  '--vault-warning',       '--vault-bg-raised'],
    ['--vault-info on --vault-bg-raised',                     '--vault-info',          '--vault-bg-raised'],
  ];

  for (const [label, fgToken, bgToken] of pairs) {
    it(label, () => {
      const r = contrastRatio(tok(fgToken), tok(bgToken));
      expect(
        r,
        `${label}: ${r.toFixed(2)}:1 < 3.0:1 (WCAG AA large text / UI element). ` +
          `fg=${tok(fgToken)} bg=${tok(bgToken)}`,
      ).toBeGreaterThanOrEqual(3.0);
    });
  }
});

// ---------------------------------------------------------------------------
// Structural: token presence guard (catches accidental token renames)
// ---------------------------------------------------------------------------

describe('#320 WCAG AA — token presence', () => {
  const required = [
    '--vault-fg',
    '--vault-fg-secondary',
    '--vault-fg-muted',
    '--vault-fg-inverse',
    '--vault-bg',
    '--vault-bg-raised',
    '--vault-bg-overlay',
    '--vault-sapphire',
    '--vault-sapphire-dark',
    '--vault-success',
    '--vault-danger',
    '--vault-warning',
    '--vault-info',
  ];

  it('all audited primitive tokens are defined in index.css', () => {
    const missing = required.filter((t) => !tokens.has(t));
    expect(
      missing,
      `these tokens were expected in index.css but are missing: ${missing.join(', ')}`,
    ).toEqual([]);
  });
});

// ---------------------------------------------------------------------------
// Structural: focus-visible declarations for key interactive elements
// ---------------------------------------------------------------------------

describe('#320 WCAG AA — focus-visible rings (AC2)', () => {
  it('Layout.css: .tab has :focus-visible rule', () => {
    const css = readFileSync(join(SRC_DIR, 'components/Layout.css'), 'utf8');
    expect(css).toMatch(/\.tab:focus-visible/);
  });

  it('Layout.css: .sub-tab has :focus-visible rule', () => {
    const css = readFileSync(join(SRC_DIR, 'components/Layout.css'), 'utf8');
    expect(css).toMatch(/\.sub-tab:focus-visible/);
  });

  it('Layout.css: .nav-brand has :focus-visible rule', () => {
    const css = readFileSync(join(SRC_DIR, 'components/Layout.css'), 'utf8');
    expect(css).toMatch(/\.nav-brand:focus-visible/);
  });

  it('EmptyState.css: .empty-state-cta has :focus-visible rule', () => {
    const css = readFileSync(join(SRC_DIR, 'components/EmptyState.css'), 'utf8');
    expect(css).toMatch(/\.empty-state-cta:focus-visible/);
  });

  it('AuthBar.css: .auth-btn has :focus-visible rule', () => {
    const css = readFileSync(join(SRC_DIR, 'components/AuthBar.css'), 'utf8');
    expect(css).toMatch(/\.auth-btn:focus-visible/);
  });

  it('App.css: global button has :focus-visible rule', () => {
    const css = readFileSync(join(SRC_DIR, 'App.css'), 'utf8');
    expect(css).toMatch(/button:focus-visible/);
  });
});

// ---------------------------------------------------------------------------
// Structural: auth-btn-signup uses --fg-inverse (not --fg) on --accent bg (AC1)
// ---------------------------------------------------------------------------

describe('#320 WCAG AA — auth-btn-signup text color', () => {
  it('AuthBar.css: .auth-btn-signup uses var(--fg-inverse), not var(--fg)', () => {
    const css = readFileSync(join(SRC_DIR, 'components/AuthBar.css'), 'utf8');
    // Find the .auth-btn-signup block
    const idx = css.indexOf('.auth-btn-signup ');
    expect(idx, '.auth-btn-signup rule not found').toBeGreaterThan(-1);
    const block = css.slice(idx, css.indexOf('}', idx) + 1);
    // Must use --fg-inverse, not the too-light --fg
    expect(block).toMatch(/color\s*:\s*var\(--fg-inverse\)/);
    expect(block).not.toMatch(/color\s*:\s*var\(--fg\)/);
  });
});
