/**
 * Design-token bridge guard test — #309 Pass A.
 *
 * Pass A ports the design-system SEMANTIC LAYER (--fg/--bg/--accent/--border/…)
 * into index.css as the canonical vocabulary, and adds a TEMPORARY legacy-alias
 * bridge so the ~600 component references to undefined legacy names
 * (--text-primary, --border-color, --accent-color, …) resolve to the new
 * sapphire palette instead of their hardcoded hex fallbacks.
 *
 * This test is the executable enforcement of that contract:
 *   1. Every custom property referenced by component CSS via var() is DEFINED
 *      somewhere in index.css — except a documented allowlist of genuinely
 *      categorical / data-viz accents that Pass A intentionally leaves alone
 *      (Ray ruling #2 / Option B).
 *   2. Every alias/token DEFINED in index.css whose value is itself a var()
 *      points at a property that is also defined (no dangling pointer — catches
 *      a bridge entry whose target token was renamed or removed).
 *
 * When the Pass-B follow-up rewrites call-sites onto the canonical names and
 * deletes the bridge, this test continues to guarantee no reference dangles.
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
 * are intentionally NOT bridged to the brand palette and keep their per-site
 * hex fallbacks. Listed explicitly so the omission is deliberate, not accidental.
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

describe('#309 Pass A — design-token bridge', () => {
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
      '--success',
      '--danger',
      '--warning',
      '--info',
    ]) {
      expect(defined.has(token), `semantic token ${token} must be defined in index.css`).toBe(true);
    }
  });

  it('bridges every legacy alias used by component CSS (except categorical accents)', () => {
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
      `these custom properties are referenced by component CSS but not defined in index.css ` +
        `(would render via hardcoded hex fallback / pre-#298 palette): ${unresolved.join(', ')}`,
    ).toEqual([]);
  });

  it('has no dangling bridge pointer (every var()-valued definition targets a defined property)', () => {
    // Properties whose value is a single var(--target) reference. The target
    // must itself be defined in index.css.
    const re = /^\s*(--[a-zA-Z0-9-]+)\s*:\s*var\(\s*(--[a-zA-Z0-9-]+)/gm;
    const dangling: string[] = [];
    let m: RegExpExecArray | null;
    while ((m = re.exec(indexCss)) !== null) {
      const [, name, target] = m;
      if (!defined.has(target)) dangling.push(`${name} -> ${target}`);
    }
    expect(dangling, `bridge pointers with undefined targets: ${dangling.join(', ')}`).toEqual([]);
  });
});
