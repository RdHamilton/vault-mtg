/**
 * Unit tests for the reportError helper (frontend/src/lib/sentry.ts).
 *
 * reportError mirrors the BFF ReportError convention:
 *   - tag keys "component" and "action" match across both layers
 *   - purely additive: never rethrows
 *   - null/undefined err is a no-op
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import * as Sentry from '@sentry/react';
import { reportError } from './sentry';

vi.mock('@sentry/react', () => ({
  captureException: vi.fn(),
}));

describe('reportError', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('calls Sentry.captureException with component and action tags', () => {
    const err = new Error('boom');
    reportError(err, { component: 'Foo', action: 'bar' });

    expect(Sentry.captureException).toHaveBeenCalledOnce();
    expect(Sentry.captureException).toHaveBeenCalledWith(err, {
      tags: { component: 'Foo', action: 'bar' },
    });
  });

  it('includes extra when provided', () => {
    const err = new Error('extra test');
    reportError(err, { component: 'Foo', action: 'bar', extra: { deckId: 'abc-123' } });

    expect(Sentry.captureException).toHaveBeenCalledWith(err, {
      tags: { component: 'Foo', action: 'bar' },
      extra: { deckId: 'abc-123' },
    });
  });

  it('omits the extra key entirely when extra is undefined', () => {
    const err = new Error('no extra');
    reportError(err, { component: 'Foo', action: 'bar' });

    const callArg = (Sentry.captureException as ReturnType<typeof vi.fn>).mock.calls[0][1] as Record<string, unknown>;
    expect(callArg).not.toHaveProperty('extra');
  });

  it('is a no-op when err is null', () => {
    reportError(null, { component: 'Foo', action: 'bar' });
    expect(Sentry.captureException).not.toHaveBeenCalled();
  });

  it('is a no-op when err is undefined', () => {
    reportError(undefined, { component: 'Foo', action: 'bar' });
    expect(Sentry.captureException).not.toHaveBeenCalled();
  });

  it('accepts non-Error values (plain string)', () => {
    reportError('string error', { component: 'Foo', action: 'bar' });
    expect(Sentry.captureException).toHaveBeenCalledOnce();
    expect(Sentry.captureException).toHaveBeenCalledWith('string error', expect.objectContaining({
      tags: { component: 'Foo', action: 'bar' },
    }));
  });
});
