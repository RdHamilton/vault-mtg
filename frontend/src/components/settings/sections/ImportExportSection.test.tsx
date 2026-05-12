import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { ImportExportSection } from './ImportExportSection';

describe('ImportExportSection', () => {
  const defaultProps = {
    onExportData: vi.fn(),
  };

  it('renders section title', () => {
    render(<ImportExportSection {...defaultProps} />);
    expect(screen.getByText('Export')).toBeInTheDocument();
  });

  it('renders section description', () => {
    render(<ImportExportSection {...defaultProps} />);
    expect(
      screen.getByText(/Export your match history for backup or external analysis/)
    ).toBeInTheDocument();
  });

  describe('export buttons', () => {
    it('renders export to JSON button', () => {
      render(<ImportExportSection {...defaultProps} />);
      expect(screen.getByRole('button', { name: 'Export to JSON' })).toBeInTheDocument();
    });

    it('renders export to CSV button', () => {
      render(<ImportExportSection {...defaultProps} />);
      expect(screen.getByRole('button', { name: 'Export to CSV' })).toBeInTheDocument();
    });

    it('calls onExportData with json when JSON button clicked', () => {
      const onExportData = vi.fn();
      render(<ImportExportSection {...defaultProps} onExportData={onExportData} />);

      fireEvent.click(screen.getByRole('button', { name: 'Export to JSON' }));

      expect(onExportData).toHaveBeenCalledWith('json');
    });

    it('calls onExportData with csv when CSV button clicked', () => {
      const onExportData = vi.fn();
      render(<ImportExportSection {...defaultProps} onExportData={onExportData} />);

      fireEvent.click(screen.getByRole('button', { name: 'Export to CSV' }));

      expect(onExportData).toHaveBeenCalledWith('csv');
    });
  });

  describe('labels and descriptions', () => {
    it('renders export data label', () => {
      render(<ImportExportSection {...defaultProps} />);
      expect(screen.getByText('Export Data')).toBeInTheDocument();
    });
  });
});
