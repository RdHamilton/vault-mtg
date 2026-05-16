import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { DangerZoneSection } from './DangerZoneSection';

/**
 * DangerZoneSection tests — covers the standalone Danger Zone accordion
 * section extracted from DataRecoverySection in #2027.
 *
 * All uninstall tests were previously in DataRecoverySection.test.tsx and
 * are now maintained here as the single source of truth for uninstall UI.
 */

describe('DangerZoneSection', () => {
  // ---------------------------------------------------------------------------
  // AC1: DangerZoneSection is its own component and renders its own title
  // ---------------------------------------------------------------------------

  describe('AC1 — standalone section with its own title', () => {
    it('renders null when onUninstallDaemon is not provided', () => {
      const { container } = render(
        <DangerZoneSection isConnected={true} />,
      );
      expect(container.firstChild).toBeNull();
    });

    it('renders the section title when onUninstallDaemon is provided', () => {
      render(
        <DangerZoneSection isConnected={true} onUninstallDaemon={vi.fn()} />,
      );
      expect(screen.getByText(/Danger Zone/i)).toBeInTheDocument();
    });

    it('renders the section data-testid', () => {
      render(
        <DangerZoneSection isConnected={true} onUninstallDaemon={vi.fn()} />,
      );
      expect(screen.getByTestId('danger-zone-section')).toBeInTheDocument();
    });

    it('renders the section description', () => {
      render(
        <DangerZoneSection isConnected={true} onUninstallDaemon={vi.fn()} />,
      );
      expect(screen.getByText(/Stop the local daemon and remove its startup entry/i)).toBeInTheDocument();
    });
  });

  // ---------------------------------------------------------------------------
  // AC3 — uninstall functionality is unchanged: all existing flow tests
  // ---------------------------------------------------------------------------

  describe('AC3 — uninstall flow unchanged', () => {
    it('renders the uninstall button when onUninstallDaemon is provided', () => {
      render(
        <DangerZoneSection isConnected={true} onUninstallDaemon={vi.fn()} />,
      );
      expect(screen.getByRole('button', { name: /Uninstall VaultMTG Daemon/i })).toBeInTheDocument();
    });

    it('disables the uninstall button when daemon is not connected', () => {
      render(
        <DangerZoneSection isConnected={false} onUninstallDaemon={vi.fn()} />,
      );
      expect(screen.getByRole('button', { name: /Uninstall VaultMTG Daemon/i })).toBeDisabled();
    });

    it('shows daemon-must-be-running hint when not connected', () => {
      render(
        <DangerZoneSection isConnected={false} onUninstallDaemon={vi.fn()} />,
      );
      expect(screen.getByText(/Daemon must be running to trigger uninstall/i)).toBeInTheDocument();
    });

    it('enables the uninstall button when connected', () => {
      render(
        <DangerZoneSection isConnected={true} onUninstallDaemon={vi.fn()} />,
      );
      expect(screen.getByRole('button', { name: /Uninstall VaultMTG Daemon/i })).not.toBeDisabled();
    });

    it('does NOT show daemon hint when connected', () => {
      render(
        <DangerZoneSection isConnected={true} onUninstallDaemon={vi.fn()} />,
      );
      expect(
        screen.queryByText(/Daemon must be running to trigger uninstall/i),
      ).not.toBeInTheDocument();
    });

    it('shows the confirmation panel after clicking Uninstall', () => {
      render(
        <DangerZoneSection isConnected={true} onUninstallDaemon={vi.fn()} />,
      );
      fireEvent.click(screen.getByRole('button', { name: /Uninstall VaultMTG Daemon/i }));
      expect(screen.getByRole('button', { name: /Confirm Uninstall/i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /Cancel/i })).toBeInTheDocument();
      expect(screen.getByText(/Also wipe my local config/i)).toBeInTheDocument();
    });

    it('cancel returns to the initial state', () => {
      render(
        <DangerZoneSection isConnected={true} onUninstallDaemon={vi.fn()} />,
      );
      fireEvent.click(screen.getByRole('button', { name: /Uninstall VaultMTG Daemon/i }));
      fireEvent.click(screen.getByRole('button', { name: /Cancel/i }));
      expect(screen.getByRole('button', { name: /Uninstall VaultMTG Daemon/i })).toBeInTheDocument();
      expect(screen.queryByRole('button', { name: /Confirm Uninstall/i })).not.toBeInTheDocument();
    });

    it('confirm fires onUninstallDaemon with purge=false and renders the backend message', async () => {
      const onUninstallDaemon = vi
        .fn()
        .mockResolvedValue(
          'Daemon stopped and removed from launchd. Drag VaultMTG to the Trash to remove the app bundle.',
        );
      render(
        <DangerZoneSection isConnected={true} onUninstallDaemon={onUninstallDaemon} />,
      );
      fireEvent.click(screen.getByRole('button', { name: /Uninstall VaultMTG Daemon/i }));
      fireEvent.click(screen.getByRole('button', { name: /Confirm Uninstall/i }));

      await waitFor(() => {
        expect(onUninstallDaemon).toHaveBeenCalledWith(false);
      });
      await waitFor(() => {
        expect(
          screen.getByText(/Drag VaultMTG to the Trash to remove the app bundle/i),
        ).toBeInTheDocument();
      });
    });

    it('confirm passes purge=true when the checkbox is ticked', async () => {
      const onUninstallDaemon = vi
        .fn()
        .mockResolvedValue(
          'Daemon stopped, removed from launchd, and config wiped. Drag VaultMTG to the Trash to remove the app bundle.',
        );
      render(
        <DangerZoneSection isConnected={true} onUninstallDaemon={onUninstallDaemon} />,
      );
      fireEvent.click(screen.getByRole('button', { name: /Uninstall VaultMTG Daemon/i }));
      const purgeCheckbox = screen.getByRole('checkbox', { name: /Also wipe my local config/i });
      fireEvent.click(purgeCheckbox);
      fireEvent.click(screen.getByRole('button', { name: /Confirm Uninstall/i }));

      await waitFor(() => {
        expect(onUninstallDaemon).toHaveBeenCalledWith(true);
      });
      await waitFor(() => {
        expect(screen.getByText(/config wiped/i)).toBeInTheDocument();
      });
    });

    it('falls back to a neutral message when the backend returns an empty string', async () => {
      const onUninstallDaemon = vi.fn().mockResolvedValue('');
      render(
        <DangerZoneSection isConnected={true} onUninstallDaemon={onUninstallDaemon} />,
      );
      fireEvent.click(screen.getByRole('button', { name: /Uninstall VaultMTG Daemon/i }));
      fireEvent.click(screen.getByRole('button', { name: /Confirm Uninstall/i }));

      await waitFor(() => {
        expect(screen.getByText(/Daemon uninstall scheduled/i)).toBeInTheDocument();
      });
    });

    it('renders an error message when the uninstall call rejects', async () => {
      const onUninstallDaemon = vi.fn().mockRejectedValue(new Error('boom'));
      render(
        <DangerZoneSection isConnected={true} onUninstallDaemon={onUninstallDaemon} />,
      );
      fireEvent.click(screen.getByRole('button', { name: /Uninstall VaultMTG Daemon/i }));
      fireEvent.click(screen.getByRole('button', { name: /Confirm Uninstall/i }));

      await waitFor(() => {
        expect(screen.getByText(/boom/i)).toBeInTheDocument();
      });
    });

    it('shows success result testid after successful uninstall', async () => {
      const onUninstallDaemon = vi.fn().mockResolvedValue('Daemon stopped.');
      render(
        <DangerZoneSection isConnected={true} onUninstallDaemon={onUninstallDaemon} />,
      );
      fireEvent.click(screen.getByRole('button', { name: /Uninstall VaultMTG Daemon/i }));
      fireEvent.click(screen.getByRole('button', { name: /Confirm Uninstall/i }));

      await waitFor(() => {
        expect(screen.getByTestId('danger-zone-success-result')).toBeInTheDocument();
      });
    });

    it('shows error result testid when uninstall fails', async () => {
      const onUninstallDaemon = vi.fn().mockRejectedValue(new Error('failed'));
      render(
        <DangerZoneSection isConnected={true} onUninstallDaemon={onUninstallDaemon} />,
      );
      fireEvent.click(screen.getByRole('button', { name: /Uninstall VaultMTG Daemon/i }));
      fireEvent.click(screen.getByRole('button', { name: /Confirm Uninstall/i }));

      await waitFor(() => {
        expect(screen.getByTestId('danger-zone-error-result')).toBeInTheDocument();
      });
    });
  });

  // ---------------------------------------------------------------------------
  // AC2 — DataRecoverySection no longer contains Danger Zone
  // (tested implicitly — the DataRecoverySection test file asserts this)
  // ---------------------------------------------------------------------------
});
