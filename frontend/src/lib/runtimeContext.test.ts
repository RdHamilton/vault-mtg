import { describe, it, expect, afterEach } from 'vitest';
import { isDesktopApp } from './runtimeContext';

describe('runtimeContext', () => {
  afterEach(() => {
    // Restore window to browser context after each test
    delete (window as Window).__VAULTMTG_DESKTOP__;
  });

  describe('isDesktopApp', () => {
    it('returns false when __VAULTMTG_DESKTOP__ is not set (browser context)', () => {
      delete (window as Window).__VAULTMTG_DESKTOP__;
      expect(isDesktopApp()).toBe(false);
    });

    it('returns true when __VAULTMTG_DESKTOP__ is set to true (desktop context)', () => {
      (window as Window).__VAULTMTG_DESKTOP__ = true;
      expect(isDesktopApp()).toBe(true);
    });

    it('returns false when __VAULTMTG_DESKTOP__ is explicitly undefined', () => {
      (window as unknown as Record<string, unknown>).__VAULTMTG_DESKTOP__ = undefined;
      expect(isDesktopApp()).toBe(false);
    });
  });
});
