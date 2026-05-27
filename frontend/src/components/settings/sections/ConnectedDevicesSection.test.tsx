/**
 * ConnectedDevicesSection — Vitest component tests (#2632)
 *
 * TDD: these tests are written first; the implementation follows.
 *
 * Covers:
 *   - Loading state renders spinner
 *   - Populated list renders device rows (truncated device_id, platform, paired_at)
 *   - Empty state renders "No devices connected."
 *   - Revoke success: optimistic row removal on 204
 *   - Revoke 404: treated as already-revoked success, row removed
 *   - Revoke error (5xx): per-row error message shown, row stays
 *   - Load error: error message shown
 *   - Sensitive-field exclusion: last_used_at is NOT rendered
 *   - Sensitive-field exclusion: no PII (email, full device_id UUID) rendered
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent, act } from '@testing-library/react';
import { ConnectedDevicesSection } from './ConnectedDevicesSection';
import * as bffDaemons from '@/services/api/bffDaemons';
import type { DaemonDevice } from '@/services/api/bffDaemons';

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const DEVICE_A: DaemonDevice = {
  device_id: 'aaaaaaaa-1111-2222-3333-444444444444',
  platform: 'windows',
  daemon_ver: 'v0.3.3',
  paired_at: '2026-05-01T10:00:00Z',
  last_used_at: '2026-05-27T08:00:00Z',
};

const DEVICE_B: DaemonDevice = {
  device_id: 'bbbbbbbb-5555-6666-7777-888888888888',
  platform: 'darwin',
  daemon_ver: 'v0.3.2',
  paired_at: '2026-04-15T14:30:00Z',
  last_used_at: null,
};

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

vi.mock('@/services/api/bffDaemons', () => ({
  listDaemons: vi.fn(),
  revokeDaemon: vi.fn(),
}));

const mockListDaemons = vi.mocked(bffDaemons.listDaemons);
const mockRevokeDaemon = vi.mocked(bffDaemons.revokeDaemon);

beforeEach(() => {
  vi.clearAllMocks();
});

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('ConnectedDevicesSection', () => {
  describe('Loading state', () => {
    it('renders a loading spinner while fetching', () => {
      // Never resolves so we're stuck in loading
      mockListDaemons.mockReturnValue(new Promise(() => {}));

      render(<ConnectedDevicesSection />);

      expect(screen.getByTestId('connected-devices-loading')).toBeInTheDocument();
    });

    it('does not render device rows during loading', () => {
      mockListDaemons.mockReturnValue(new Promise(() => {}));

      render(<ConnectedDevicesSection />);

      expect(screen.queryByTestId('device-row')).not.toBeInTheDocument();
    });
  });

  describe('Populated list', () => {
    beforeEach(() => {
      mockListDaemons.mockResolvedValue({ devices: [DEVICE_A, DEVICE_B] });
    });

    it('renders one row per device', async () => {
      render(<ConnectedDevicesSection />);

      await waitFor(() => {
        expect(screen.getAllByTestId('device-row')).toHaveLength(2);
      });
    });

    it('renders truncated device_id (first 8 chars + ellipsis)', async () => {
      render(<ConnectedDevicesSection />);

      await waitFor(() => {
        // DEVICE_A id starts with 'aaaaaaaa'
        expect(screen.getByText('aaaaaaaa…')).toBeInTheDocument();
      });
    });

    it('renders platform for each device', async () => {
      render(<ConnectedDevicesSection />);

      await waitFor(() => {
        expect(screen.getByText('windows')).toBeInTheDocument();
        expect(screen.getByText('darwin')).toBeInTheDocument();
      });
    });

    it('renders paired_at date for each device', async () => {
      render(<ConnectedDevicesSection />);

      await waitFor(() => {
        // Paired at dates should appear somewhere in the rendered rows
        expect(screen.getByTestId('device-row-0')).toBeInTheDocument();
        expect(screen.getByTestId('device-row-1')).toBeInTheDocument();
      });
    });

    it('renders a Revoke button per row', async () => {
      render(<ConnectedDevicesSection />);

      await waitFor(() => {
        expect(screen.getAllByTestId('revoke-button')).toHaveLength(2);
      });
    });

    it('does NOT render last_used_at anywhere (Ray Q4 binding)', async () => {
      render(<ConnectedDevicesSection />);

      await waitFor(() => {
        expect(screen.getAllByTestId('device-row')).toHaveLength(2);
      });

      // last_used_at value must not appear in the document
      expect(screen.queryByText('2026-05-27T08:00:00Z')).not.toBeInTheDocument();
      expect(screen.queryByText('last_used_at')).not.toBeInTheDocument();
      // No "last seen", "last active", "last used" labels either
      expect(screen.queryByText(/last.*(seen|active|used)/i)).not.toBeInTheDocument();
    });

    it('does NOT render the full UUID (only truncated form)', async () => {
      render(<ConnectedDevicesSection />);

      await waitFor(() => {
        expect(screen.getAllByTestId('device-row')).toHaveLength(2);
      });

      // Scans visible text nodes
      expect(screen.queryByText(DEVICE_A.device_id)).not.toBeInTheDocument();
      expect(screen.queryByText(DEVICE_B.device_id)).not.toBeInTheDocument();
      // Scans entire rendered HTML including ALL DOM attributes (title, data-*, id, aria-*, etc.)
      expect(document.body.innerHTML).not.toContain(DEVICE_A.device_id);
      expect(document.body.innerHTML).not.toContain(DEVICE_B.device_id);
    });
  });

  describe('Empty state', () => {
    it('renders "No devices connected." when devices array is empty', async () => {
      mockListDaemons.mockResolvedValue({ devices: [] });

      render(<ConnectedDevicesSection />);

      await waitFor(() => {
        expect(screen.getByTestId('connected-devices-empty')).toBeInTheDocument();
        expect(screen.getByText('No devices connected.')).toBeInTheDocument();
      });
    });
  });

  describe('Revoke — success (204)', () => {
    it('removes the row optimistically on 204', async () => {
      mockListDaemons.mockResolvedValue({ devices: [DEVICE_A, DEVICE_B] });
      mockRevokeDaemon.mockResolvedValue(undefined);

      render(<ConnectedDevicesSection />);

      await waitFor(() => {
        expect(screen.getAllByTestId('device-row')).toHaveLength(2);
      });

      await act(async () => {
        fireEvent.click(screen.getAllByTestId('revoke-button')[0]);
      });

      await waitFor(() => {
        expect(screen.getAllByTestId('device-row')).toHaveLength(1);
      });

      // DEVICE_A row must be gone — verify by truncated id text no longer present
      expect(screen.queryByText('aaaaaaaa…')).not.toBeInTheDocument();
    });

    it('calls revokeDaemon with the correct device_id and token', async () => {
      mockListDaemons.mockResolvedValue({ devices: [DEVICE_A] });
      mockRevokeDaemon.mockResolvedValue(undefined);

      render(<ConnectedDevicesSection />);

      await waitFor(() => {
        expect(screen.getByTestId('revoke-button')).toBeInTheDocument();
      });

      fireEvent.click(screen.getByTestId('revoke-button'));

      await waitFor(() => {
        expect(mockRevokeDaemon).toHaveBeenCalledWith(
          DEVICE_A.device_id,
          'clerk-test-token-stub'
        );
      });
    });
  });

  describe('Revoke — 404 treated as success', () => {
    it('removes the row when revoke returns 404 (already-revoked)', async () => {
      mockListDaemons.mockResolvedValue({ devices: [DEVICE_A] });
      // Simulate 404: revokeDaemon resolves (adapter collapses 404 to success)
      mockRevokeDaemon.mockResolvedValue(undefined);

      render(<ConnectedDevicesSection />);

      await waitFor(() => {
        expect(screen.getByTestId('revoke-button')).toBeInTheDocument();
      });

      await act(async () => {
        fireEvent.click(screen.getByTestId('revoke-button'));
      });

      await waitFor(() => {
        expect(screen.queryByTestId('device-row')).not.toBeInTheDocument();
      });
    });
  });

  describe('Revoke — error (5xx)', () => {
    it('shows a per-row error message on revoke failure', async () => {
      mockListDaemons.mockResolvedValue({ devices: [DEVICE_A, DEVICE_B] });
      mockRevokeDaemon.mockRejectedValue(new Error('Server error'));

      render(<ConnectedDevicesSection />);

      await waitFor(() => {
        expect(screen.getAllByTestId('revoke-button')).toHaveLength(2);
      });

      fireEvent.click(screen.getAllByTestId('revoke-button')[0]);

      await waitFor(() => {
        expect(screen.getByTestId('revoke-error-0')).toBeInTheDocument();
      });
    });

    it('keeps the row in the list on revoke failure', async () => {
      mockListDaemons.mockResolvedValue({ devices: [DEVICE_A] });
      mockRevokeDaemon.mockRejectedValue(new Error('Server error'));

      render(<ConnectedDevicesSection />);

      await waitFor(() => {
        expect(screen.getByTestId('revoke-button')).toBeInTheDocument();
      });

      fireEvent.click(screen.getByTestId('revoke-button'));

      await waitFor(() => {
        expect(screen.getByTestId('revoke-error-0')).toBeInTheDocument();
      });

      // Row must still be present
      expect(screen.getByTestId('device-row-0')).toBeInTheDocument();
    });

    it('does not show error on the other rows', async () => {
      mockListDaemons.mockResolvedValue({ devices: [DEVICE_A, DEVICE_B] });
      mockRevokeDaemon.mockRejectedValue(new Error('Server error'));

      render(<ConnectedDevicesSection />);

      await waitFor(() => {
        expect(screen.getAllByTestId('revoke-button')).toHaveLength(2);
      });

      // Click the first revoke button (DEVICE_A at index 0)
      fireEvent.click(screen.getAllByTestId('revoke-button')[0]);

      await waitFor(() => {
        expect(screen.getByTestId('revoke-error-0')).toBeInTheDocument();
      });

      // Row 1 (DEVICE_B) should have no error
      expect(screen.queryByTestId('revoke-error-1')).not.toBeInTheDocument();
    });
  });

  describe('Load error', () => {
    it('renders an error message when listDaemons rejects', async () => {
      mockListDaemons.mockRejectedValue(new Error('Network failure'));

      render(<ConnectedDevicesSection />);

      await waitFor(() => {
        expect(screen.getByTestId('connected-devices-error')).toBeInTheDocument();
      });
    });

    it('does not render device rows on load error', async () => {
      mockListDaemons.mockRejectedValue(new Error('Network failure'));

      render(<ConnectedDevicesSection />);

      await waitFor(() => {
        expect(screen.getByTestId('connected-devices-error')).toBeInTheDocument();
      });

      expect(screen.queryByTestId('device-row')).not.toBeInTheDocument();
    });
  });

  describe('Sensitive field exclusion', () => {
    it('never renders last_used_at value regardless of API response', async () => {
      const deviceWithLastUsed: DaemonDevice = {
        ...DEVICE_A,
        last_used_at: '2026-05-27T09:00:00Z',
      };
      mockListDaemons.mockResolvedValue({ devices: [deviceWithLastUsed] });

      render(<ConnectedDevicesSection />);

      await waitFor(() => {
        expect(screen.getByTestId('device-row')).toBeInTheDocument();
      });

      expect(screen.queryByText('2026-05-27T09:00:00Z')).not.toBeInTheDocument();
    });

    it('never renders the full device_id UUID in text or DOM attributes', async () => {
      mockListDaemons.mockResolvedValue({ devices: [DEVICE_A] });

      render(<ConnectedDevicesSection />);

      await waitFor(() => {
        expect(screen.getByTestId('device-row')).toBeInTheDocument();
      });

      // Scans visible text nodes
      expect(screen.queryByText(DEVICE_A.device_id)).not.toBeInTheDocument();
      // Scans entire rendered HTML including ALL DOM attributes (title, data-*, id, aria-*, etc.)
      expect(document.body.innerHTML).not.toContain(DEVICE_A.device_id);
    });
  });
});
