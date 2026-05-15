import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
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

  // Danger Zone tests have been moved to DangerZoneSection.test.tsx
  // as part of the #2027 refactor — DataRecoverySection no longer owns uninstall UI.
});
