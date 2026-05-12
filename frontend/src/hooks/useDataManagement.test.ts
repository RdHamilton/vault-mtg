import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useDataManagement } from './useDataManagement';

// Mock the API modules
vi.mock('@/services/api', () => ({
  matches: {
    exportMatches: vi.fn(),
  },
}));

// Mock download utility
vi.mock('@/utils/download', () => ({
  downloadTextFile: vi.fn(),
}));

// Mock showToast
vi.mock('../components/ToastContainer', () => ({
  showToast: {
    show: vi.fn(),
  },
}));

import { matches } from '@/services/api';
import { downloadTextFile } from '@/utils/download';
import { showToast } from '../components/ToastContainer';

const mockExportMatches = vi.mocked(matches.exportMatches);
const mockDownloadTextFile = vi.mocked(downloadTextFile);

describe('useDataManagement', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockExportMatches.mockResolvedValue([]);
  });

  describe('handleExportData', () => {
    it('exports to JSON when format is json', async () => {
      mockExportMatches.mockResolvedValueOnce([{ id: 1 }]);

      const { result } = renderHook(() => useDataManagement());

      await act(async () => {
        await result.current.handleExportData('json');
      });

      expect(mockExportMatches).toHaveBeenCalledWith('json');
      expect(mockDownloadTextFile).toHaveBeenCalledWith(
        expect.any(String),
        'mtga-matches.json'
      );
    });

    it('exports to CSV when format is csv', async () => {
      mockExportMatches.mockResolvedValueOnce('csv,data');

      const { result } = renderHook(() => useDataManagement());

      await act(async () => {
        await result.current.handleExportData('csv');
      });

      expect(mockExportMatches).toHaveBeenCalledWith('csv');
      expect(mockDownloadTextFile).toHaveBeenCalledWith(
        'csv,data',
        'mtga-matches.csv'
      );
    });

    it('shows success toast after successful export', async () => {
      const { result } = renderHook(() => useDataManagement());

      await act(async () => {
        await result.current.handleExportData('json');
      });

      expect(showToast.show).toHaveBeenCalledWith(
        expect.stringContaining('Successfully exported'),
        'success'
      );
    });

    it('shows error toast when export fails', async () => {
      mockExportMatches.mockRejectedValueOnce(new Error('export failed'));

      const { result } = renderHook(() => useDataManagement());

      await act(async () => {
        await result.current.handleExportData('json');
      });

      expect(showToast.show).toHaveBeenCalledWith(
        expect.stringContaining('Failed to export'),
        'error'
      );
    });
  });
});
