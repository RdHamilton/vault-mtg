import { test, expect } from '@playwright/test';

/**
 * OG / Twitter meta-tag and GA4 guard E2E tests (#1478)
 *
 * Verifies that all 9 required Open Graph and Twitter Card meta tags are
 * present in the document <head> with non-empty content values, and that
 * the GA4 gtag script is NOT injected when VITE_GA4_MEASUREMENT_ID is unset
 * (the no-op guard that runs in dev/staging/CI environments).
 *
 * These tests run against the local dev server (baseURL from playwright.config.ts)
 * and do not require authentication — meta tags live in index.html.
 */

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL ?? 'http://localhost:3000';

test.describe('@smoke OG / Twitter meta tags', () => {
  const OG_TAGS = [
    'og:title',
    'og:description',
    'og:image',
    'og:url',
    'og:type',
  ] as const;

  const TWITTER_TAGS = [
    'twitter:card',
    'twitter:title',
    'twitter:description',
    'twitter:image',
  ] as const;

  test('all 9 OG and Twitter meta tags are present with non-empty content', async ({ page }) => {
    await page.goto(BASE_URL);

    // Check Open Graph property tags
    for (const property of OG_TAGS) {
      const content = await page
        .locator(`meta[property="${property}"]`)
        .getAttribute('content');

      expect(
        content,
        `Expected meta[property="${property}"] to have non-empty content`
      ).toBeTruthy();
    }

    // Check Twitter name tags
    for (const name of TWITTER_TAGS) {
      const content = await page
        .locator(`meta[name="${name}"]`)
        .getAttribute('content');

      expect(
        content,
        `Expected meta[name="${name}"] to have non-empty content`
      ).toBeTruthy();
    }
  });

  test('GA4 no-op guard: no gtag script with G- measurement ID in DOM when env var is unset', async ({ page }) => {
    await page.goto(BASE_URL);

    // Collect all script src attributes from the DOM
    const scriptSrcs = await page.evaluate(() => {
      return Array.from(document.querySelectorAll('script[src]')).map(
        (el) => el.getAttribute('src') ?? ''
      );
    });

    // None of the injected script src values should contain a G- measurement ID
    const gtagScripts = scriptSrcs.filter((src) =>
      src.includes('googletagmanager.com/gtag/js') && src.includes('G-')
    );

    expect(
      gtagScripts,
      `GA4 gtag script with G- ID should not be present when VITE_GA4_MEASUREMENT_ID is unset, found: ${gtagScripts.join(', ')}`
    ).toHaveLength(0);
  });
});
