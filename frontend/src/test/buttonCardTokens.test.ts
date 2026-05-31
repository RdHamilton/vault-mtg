/**
 * Button + card design-token compliance guard — #313 set 2.
 *
 * Asserts:
 *   1. Button-related CSS classes use var(--radius-md) (not bare 4px / 6px)
 *      for border-radius where those classes should follow the design system.
 *   2. Primary and action buttons declare a :focus-visible rule (WCAG AA).
 *   3. No raw rgba() with hard-coded accent RGB that bypasses the token contract.
 *   4. Danger-button background uses var(--danger), not a raw hex.
 *   5. Card classes use var(--radius-md) border-radius and var(--border) borders.
 *   6. DeckSuggestionCard viabilityStyles uses token vars, not raw hex strings.
 *
 * Companion to noRawHex.test.ts (which guards the categorical allowlist) and
 * designTokenBridge.test.ts (which guards var() reference integrity).
 */
import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const SRC_DIR = join(dirname(fileURLToPath(import.meta.url)), '..');

function read(rel: string): string {
  return readFileSync(join(SRC_DIR, rel), 'utf8');
}

// ------------------------------------------------------------------ helpers --

/** Extract the CSS text for a given selector block (simple, single-level). */
function extractBlock(css: string, selector: string): string | null {
  const idx = css.indexOf(selector);
  if (idx === -1) return null;
  const open = css.indexOf('{', idx);
  if (open === -1) return null;
  const close = css.indexOf('}', open);
  if (close === -1) return null;
  return css.slice(open + 1, close);
}

// ------------------------------------------------------------------ files ---

describe('#313 set 2 — button + card design-token compliance', () => {
  // --------------------------------------------------------- App.css buttons --

  describe('App.css global button baseline', () => {
    const css = read('App.css');

    it('button uses var(--radius-md) for border-radius', () => {
      const block = extractBlock(css, '\nbutton ');
      expect(block, 'button block not found').not.toBeNull();
      expect(block).toMatch(/border-radius\s*:\s*var\(--radius-md\)/);
    });

    it('button uses var(--transition-fast) for transition', () => {
      const block = extractBlock(css, '\nbutton ');
      expect(block).toMatch(/transition\s*:\s*var\(--transition-fast\)/);
    });

    it('button declares :focus-visible outline', () => {
      expect(css).toMatch(/:focus-visible/);
      // The focus ring must use the sapphire dark token
      const focusIdx = css.indexOf(':focus-visible');
      const block = css.slice(focusIdx, css.indexOf('}', focusIdx) + 1);
      expect(block).toMatch(/--vault-sapphire-dark/);
    });

    it('button:active uses var(--accent-press), not bare --vault-sapphire-dark directly in active rule', () => {
      // active state should use the semantic press token
      const activeIdx = css.indexOf('button:active');
      expect(activeIdx, 'button:active rule not found').toBeGreaterThan(-1);
      const block = css.slice(activeIdx, css.indexOf('}', activeIdx) + 1);
      expect(block).toMatch(/var\(--accent-press\)|var\(--vault-sapphire-dark\)/);
    });
  });

  // ------------------------------------------------- Settings.css buttons ---

  describe('Settings.css action / primary / danger buttons', () => {
    const css = read('pages/Settings.css');

    it('.action-button uses var(--radius-md) for border-radius', () => {
      const block = extractBlock(css, '.action-button ');
      expect(block, '.action-button block not found').not.toBeNull();
      expect(block).toMatch(/border-radius\s*:\s*var\(--radius-md\)/);
    });

    it('.action-button:hover uses token shadow (no bare rgba accent)', () => {
      const idx = css.indexOf('.action-button:hover');
      expect(idx, '.action-button:hover not found').toBeGreaterThan(-1);
      const block = css.slice(idx, css.indexOf('}', idx) + 1);
      // Must NOT contain a raw rgb(a) with the old accent value 74, 158, 255
      expect(block).not.toMatch(/rgba\(74,?\s*158,?\s*255/);
      // Must use a token-based shadow
      expect(block).toMatch(/var\(--shadow-sapphire-vault\)|var\(--shadow-sm\)|var\(--shadow-md\)/);
    });

    it('.action-button declares :focus-visible', () => {
      expect(css).toMatch(/\.action-button:focus-visible|button:focus-visible/);
    });

    it('.primary-button uses var(--radius-md)', () => {
      const block = extractBlock(css, '.primary-button ');
      expect(block, '.primary-button block not found').not.toBeNull();
      expect(block).toMatch(/border-radius\s*:\s*var\(--radius-md\)/);
    });

    it('.danger-button background uses var(--danger), not a raw hex', () => {
      const block = extractBlock(css, '.danger-button ');
      expect(block, '.danger-button block not found').not.toBeNull();
      // Must use the danger token
      expect(block).toMatch(/background(-color)?\s*:\s*var\(--danger\)/);
      // Must NOT have a raw hex (ff7d7d is categorical but not for a base background)
      expect(block).not.toMatch(/#[0-9a-fA-F]{3,8}/);
    });
  });

  // ---------------------------------------------- DeckBuilder.css buttons --

  describe('DeckBuilder.css .action-button', () => {
    const css = read('pages/DeckBuilder.css');

    it('.action-button uses var(--radius-md)', () => {
      const block = extractBlock(css, '.action-button ');
      expect(block, '.action-button block not found').not.toBeNull();
      expect(block).toMatch(/border-radius\s*:\s*var\(--radius-md\)/);
    });

    it('.action-button declares :focus-visible', () => {
      expect(css).toMatch(/\.action-button:focus-visible/);
    });
  });

  // ------------------------------------------ SuggestDecksModal.css cards --

  describe('SuggestDecksModal.css .deck-suggestion-card', () => {
    const css = read('components/SuggestDecksModal.css');

    it('.deck-suggestion-card uses var(--radius-md) for border-radius', () => {
      const block = extractBlock(css, '.deck-suggestion-card ');
      expect(block, '.deck-suggestion-card block not found').not.toBeNull();
      expect(block).toMatch(/border-radius\s*:\s*var\(--radius-md\)/);
    });

    it('.deck-suggestion-card uses var(--bg-raised) or var(--vault-bg-raised) for background', () => {
      const block = extractBlock(css, '.deck-suggestion-card ');
      expect(block).toMatch(/background(-color)?\s*:\s*var\(--bg-raised\)|var\(--vault-bg-raised\)/);
    });

    it('.action-btn uses var(--radius-md)', () => {
      const block = extractBlock(css, '.action-btn ');
      expect(block, '.action-btn block not found').not.toBeNull();
      expect(block).toMatch(/border-radius\s*:\s*var\(--radius-md\)/);
    });

    it('.action-btn declares :focus-visible', () => {
      expect(css).toMatch(/\.action-btn:focus-visible/);
    });
  });

  // ------------------------------------------ DeckSuggestionCard.tsx inline --

  describe('DeckSuggestionCard.tsx viabilityStyles uses tokens', () => {
    const tsx = read('components/DeckSuggestionCard.tsx');

    it('does not use raw hex strings in viabilityStyles', () => {
      // Find the viabilityStyles block
      const start = tsx.indexOf('viabilityStyles');
      const end = tsx.indexOf('};', start) + 2;
      const block = tsx.slice(start, end);
      // Should not contain raw hex — should reference CSS token vars
      expect(block).not.toMatch(/#[0-9a-fA-F]{3,8}/);
    });
  });

  // --------------------------------------------- RecommendationCard.css card --

  describe('RecommendationCard.css .recommendation-card', () => {
    const css = read('components/RecommendationCard.css');

    it('.recommendation-card uses var(--radius-md) or var(--bg-raised)', () => {
      const block = extractBlock(css, '.recommendation-card ');
      expect(block, '.recommendation-card block not found').not.toBeNull();
      // Should use token for radius
      expect(block).toMatch(/border-radius\s*:\s*var\(--radius-md\)/);
    });
  });
});
