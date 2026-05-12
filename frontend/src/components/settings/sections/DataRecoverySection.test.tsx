import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { DataRecoverySection } from './DataRecoverySection';
import { gui } from '@/types/models';

describe('DataRecoverySection', () => {
  const defaultProps = {
    isConnected: false,
    clearDataBeforeReplay: false,
    onClearDataBeforeReplayChange: vi.fn(),
    isReplaying: false,
    replayProgress: null,
    onReplayLogs: vi.fn(),
  };

  it('renders section title', () => {
    render(<DataRecoverySection {...defaultProps} />);
    expect(screen.getByText('Data Recovery')).toBeInTheDocument();
  });

  it('renders section description', () => {
    render(<DataRecoverySection {...defaultProps} />);
    expect(screen.getByText(/Recover historical data/)).toBeInTheDocument();
  });

  describe('replay logs', () => {
    it('renders replay logs checkbox', () => {
      render(<DataRecoverySection {...defaultProps} />);
      expect(screen.getByText(/Clear all data before replay/)).toBeInTheDocument();
    });

    it('calls onClearDataBeforeReplayChange when checkbox toggled', () => {
      const onClearDataBeforeReplayChange = vi.fn();
      render(
        <DataRecoverySection
          {...defaultProps}
          onClearDataBeforeReplayChange={onClearDataBeforeReplayChange}
        />
      );

      const checkbox = screen.getByRole('checkbox');
      fireEvent.click(checkbox);

      expect(onClearDataBeforeReplayChange).toHaveBeenCalledWith(true);
    });

    it('disables checkbox when replaying', () => {
      render(<DataRecoverySection {...defaultProps} isReplaying={true} />);
      expect(screen.getByRole('checkbox')).toBeDisabled();
    });

    it('disables replay button when not connected', () => {
      render(<DataRecoverySection {...defaultProps} isConnected={false} />);
      expect(screen.getByRole('button', { name: 'Replay Historical Logs' })).toBeDisabled();
    });

    it('enables replay button when connected', () => {
      render(<DataRecoverySection {...defaultProps} isConnected={true} />);
      expect(screen.getByRole('button', { name: 'Replay Historical Logs' })).not.toBeDisabled();
    });

    it('calls onReplayLogs when replay button clicked', () => {
      const onReplayLogs = vi.fn();
      render(<DataRecoverySection {...defaultProps} isConnected={true} onReplayLogs={onReplayLogs} />);

      fireEvent.click(screen.getByRole('button', { name: 'Replay Historical Logs' }));

      expect(onReplayLogs).toHaveBeenCalled();
    });

    it('shows daemon hint when not connected', () => {
      render(<DataRecoverySection {...defaultProps} isConnected={false} />);
      expect(screen.getByText('Daemon must be running to replay logs')).toBeInTheDocument();
    });

    it('does not show daemon hint when connected', () => {
      render(<DataRecoverySection {...defaultProps} isConnected={true} />);
      expect(screen.queryByText('Daemon must be running to replay logs')).not.toBeInTheDocument();
    });
  });

  describe('replay progress', () => {
    it('does not show progress when not replaying and no progress', () => {
      render(<DataRecoverySection {...defaultProps} isReplaying={false} replayProgress={null} />);
      expect(screen.queryByText('Replaying Historical Logs...')).not.toBeInTheDocument();
    });

    it('shows progress when replaying', () => {
      const progress = new gui.LogReplayProgress({
        processedFiles: 5,
        totalFiles: 10,
        totalEntries: 1000,
        matchesImported: 50,
        decksImported: 10,
        questsImported: 5,
      });

      render(<DataRecoverySection {...defaultProps} isReplaying={true} replayProgress={progress} />);
      expect(screen.getByText('Replaying Historical Logs...')).toBeInTheDocument();
      expect(screen.getByText(/Files: 5 \/ 10/)).toBeInTheDocument();
    });

    it('shows completion message when done', () => {
      const progress = new gui.LogReplayProgress({
        processedFiles: 10,
        totalFiles: 10,
      });

      render(<DataRecoverySection {...defaultProps} isReplaying={false} replayProgress={progress} />);
      expect(screen.getByText('✓ Replay Complete')).toBeInTheDocument();
    });
  });

  describe('danger zone (uninstall daemon)', () => {
    it('hides the danger zone when onUninstallDaemon is not provided', () => {
      render(<DataRecoverySection {...defaultProps} />);
      expect(screen.queryByText(/Danger Zone/i)).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: /Uninstall VaultMTG Daemon/i })).not.toBeInTheDocument();
    });

    it('renders the uninstall button when onUninstallDaemon is provided', () => {
      render(
        <DataRecoverySection
          {...defaultProps}
          isConnected={true}
          onUninstallDaemon={vi.fn()}
        />,
      );
      expect(screen.getByText(/Danger Zone/i)).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /Uninstall VaultMTG Daemon/i })).toBeInTheDocument();
    });

    it('disables the uninstall button when daemon is not connected', () => {
      render(
        <DataRecoverySection
          {...defaultProps}
          isConnected={false}
          onUninstallDaemon={vi.fn()}
        />,
      );
      expect(screen.getByRole('button', { name: /Uninstall VaultMTG Daemon/i })).toBeDisabled();
      expect(screen.getByText(/Daemon must be running to trigger uninstall/i)).toBeInTheDocument();
    });

    it('shows the confirmation panel after clicking Uninstall', () => {
      render(
        <DataRecoverySection
          {...defaultProps}
          isConnected={true}
          onUninstallDaemon={vi.fn()}
        />,
      );
      fireEvent.click(screen.getByRole('button', { name: /Uninstall VaultMTG Daemon/i }));
      expect(screen.getByRole('button', { name: /Confirm Uninstall/i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /Cancel/i })).toBeInTheDocument();
      expect(screen.getByText(/Also wipe my local config/i)).toBeInTheDocument();
    });

    it('cancel returns to the initial state', () => {
      render(
        <DataRecoverySection
          {...defaultProps}
          isConnected={true}
          onUninstallDaemon={vi.fn()}
        />,
      );
      fireEvent.click(screen.getByRole('button', { name: /Uninstall VaultMTG Daemon/i }));
      fireEvent.click(screen.getByRole('button', { name: /Cancel/i }));
      expect(screen.getByRole('button', { name: /Uninstall VaultMTG Daemon/i })).toBeInTheDocument();
      expect(screen.queryByRole('button', { name: /Confirm Uninstall/i })).not.toBeInTheDocument();
    });

    it('confirm fires onUninstallDaemon with purge=false by default', async () => {
      const onUninstallDaemon = vi.fn().mockResolvedValue(undefined);
      render(
        <DataRecoverySection
          {...defaultProps}
          isConnected={true}
          onUninstallDaemon={onUninstallDaemon}
        />,
      );
      fireEvent.click(screen.getByRole('button', { name: /Uninstall VaultMTG Daemon/i }));
      fireEvent.click(screen.getByRole('button', { name: /Confirm Uninstall/i }));

      await waitFor(() => {
        expect(onUninstallDaemon).toHaveBeenCalledWith(false);
      });
      await waitFor(() => {
        expect(screen.getByText(/Daemon uninstall scheduled/i)).toBeInTheDocument();
      });
    });

    it('confirm passes purge=true when the checkbox is ticked', async () => {
      const onUninstallDaemon = vi.fn().mockResolvedValue(undefined);
      render(
        <DataRecoverySection
          {...defaultProps}
          isConnected={true}
          onUninstallDaemon={onUninstallDaemon}
        />,
      );
      fireEvent.click(screen.getByRole('button', { name: /Uninstall VaultMTG Daemon/i }));
      const purgeCheckbox = screen.getByRole('checkbox', { name: /Also wipe my local config/i });
      fireEvent.click(purgeCheckbox);
      fireEvent.click(screen.getByRole('button', { name: /Confirm Uninstall/i }));

      await waitFor(() => {
        expect(onUninstallDaemon).toHaveBeenCalledWith(true);
      });
    });

    it('renders an error message when the uninstall call rejects', async () => {
      const onUninstallDaemon = vi.fn().mockRejectedValue(new Error('boom'));
      render(
        <DataRecoverySection
          {...defaultProps}
          isConnected={true}
          onUninstallDaemon={onUninstallDaemon}
        />,
      );
      fireEvent.click(screen.getByRole('button', { name: /Uninstall VaultMTG Daemon/i }));
      fireEvent.click(screen.getByRole('button', { name: /Confirm Uninstall/i }));

      await waitFor(() => {
        expect(screen.getByText(/boom/i)).toBeInTheDocument();
      });
    });
  });
});
