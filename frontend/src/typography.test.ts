import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

/**
 * Brand font wiring (#310).
 *
 * These tests assert that the design-system type tokens are wired to the
 * correct element selectors in the SPA. They parse the real source files
 * (index.css + index.html) rather than relying on jsdom's getComputedStyle,
 * which does not resolve `var()` from injected stylesheets — so a source-level
 * assertion is the reliable way to lock the wiring.
 */

const indexCss = readFileSync(
  resolve(__dirname, './index.css'),
  'utf8',
);
const indexHtml = readFileSync(
  resolve(__dirname, '../index.html'),
  'utf8',
);

// Collapse whitespace so the selector/declaration assertions are
// formatting-insensitive.
const cssFlat = indexCss.replace(/\s+/g, ' ');

describe('typography token wiring (#310)', () => {
  it('defines the three brand type tokens from colors_and_type.css', () => {
    expect(indexCss).toMatch(
      /--font-display-vault:\s*"Space Grotesk"/,
    );
    expect(indexCss).toMatch(/--font-body:\s*"Inter"/);
    expect(indexCss).toMatch(/--font-mono:\s*"JetBrains Mono"/);
  });

  it('body inherits the Inter body token at :root', () => {
    expect(cssFlat).toMatch(
      /:root,\s*\[data-theme="dark"\]\s*\{[^}]*font-family:\s*var\(--font-body\)/,
    );
  });

  it('wires every heading level to the Space Grotesk display token', () => {
    // h1..h6 share one rule -> var(--font-display-vault)
    const headingRule =
      /h1,\s*h2,\s*h3,\s*h4,\s*h5,\s*h6\s*\{[^}]*font-family:\s*var\(--font-display-vault\)/;
    expect(cssFlat).toMatch(headingRule);
  });

  it('wires code / kbd / samp / pre / .mono to the JetBrains Mono token', () => {
    const monoRule =
      /code,\s*kbd,\s*samp,\s*pre,\s*\.mono\s*\{[^}]*font-family:\s*var\(--font-mono\)/;
    expect(cssFlat).toMatch(monoRule);
  });
});

describe('critical-font preload (#310)', () => {
  it('preloads exactly one font asset', () => {
    const preloads = indexHtml.match(/rel="preload"[^>]*as="font"/g) ?? [];
    expect(preloads).toHaveLength(1);
  });

  it('preloads the Space Grotesk 700 latin-subset woff2 the stylesheet resolves to', () => {
    // Pinning the exact gstatic URL ensures the hint is consumed (no
    // double-fetch / unused-preload console warning).
    expect(indexHtml).toContain(
      'https://fonts.gstatic.com/s/spacegrotesk/v22/V8mDoQDjQSkFtoMM3T6r8E7mPbF4Cw.woff2',
    );
    expect(indexHtml).toMatch(/rel="preload"[\s\S]*?type="font\/woff2"/);
    // crossorigin is mandatory for a font preload to match the CORS fetch.
    expect(indexHtml).toMatch(/rel="preload"[\s\S]*?crossorigin/);
  });

  it('keeps font-display: swap on the Google Fonts stylesheet (no CLS regression)', () => {
    expect(indexHtml).toMatch(
      /fonts\.googleapis\.com\/css2[^"]*display=swap/,
    );
  });

  it('keeps the Google Fonts CDN stylesheet (not self-hosted)', () => {
    expect(indexHtml).toMatch(
      /<link rel="stylesheet" href="https:\/\/fonts\.googleapis\.com\/css2\?family=Space\+Grotesk/,
    );
  });
});
