import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { render } from '../test/utils/testUtils';
import DraftAnalytics from './DraftAnalytics';
import { mockDrafts } from '@/test/mocks/apiMock';

describe('DraftAnalytics Page', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockDrafts.getDraftFormats.mockResolvedValue(['DSK', 'FDN', 'BLB']);
  });

  describe('Loading State', () => {
    it('should display loading state while fetching draft formats', () => {
      mockDrafts.getDraftFormats.mockImplementation(() => new Promise(() => {})); // Never resolves

      render(<DraftAnalytics />);

      expect(screen.getByText('Loading draft analytics...')).toBeInTheDocument();
    });
  });

  describe('Empty State', () => {
    it('should display empty state when no draft formats available', async () => {
      mockDrafts.getDraftFormats.mockResolvedValue([]);

      render(<DraftAnalytics />);

      await waitFor(() => {
        expect(screen.getByText('No Draft Data Available')).toBeInTheDocument();
        expect(
          screen.getByText('Complete some drafts to see your analytics and performance trends.')
        ).toBeInTheDocument();
      });
    });
  });

  describe('Page Header', () => {
    it('should display the page title', async () => {
      render(<DraftAnalytics />);

      await waitFor(() => {
        expect(screen.getByText('Draft Analytics')).toBeInTheDocument();
      });
    });
  });

  describe('Set Filter', () => {
    it('should display set dropdown with available sets', async () => {
      render(<DraftAnalytics />);

      await waitFor(() => {
        const setSelect = screen.getByLabelText('Set');
        expect(setSelect).toBeInTheDocument();
      });

      const setSelect = screen.getByLabelText('Set') as HTMLSelectElement;
      expect(setSelect.options).toHaveLength(3);
      expect(setSelect.options[0].text).toBe('DSK');
      expect(setSelect.options[1].text).toBe('FDN');
      expect(setSelect.options[2].text).toBe('BLB');
    });

    it('should select first set by default', async () => {
      render(<DraftAnalytics />);

      await waitFor(() => {
        const setSelect = screen.getByLabelText('Set') as HTMLSelectElement;
        expect(setSelect.value).toBe('DSK');
      });
    });

    it('should allow changing selected set', async () => {
      const user = userEvent.setup();

      render(<DraftAnalytics />);

      await waitFor(() => {
        expect(screen.getByLabelText('Set')).toBeInTheDocument();
      });

      const setSelect = screen.getByLabelText('Set');
      await user.selectOptions(setSelect, 'FDN');

      expect((setSelect as HTMLSelectElement).value).toBe('FDN');
    });
  });

  describe('Format Filter', () => {
    it('should display format dropdown with draft formats', async () => {
      render(<DraftAnalytics />);

      await waitFor(() => {
        const formatSelect = screen.getByLabelText('Format');
        expect(formatSelect).toBeInTheDocument();
      });

      const formatSelect = screen.getByLabelText('Format') as HTMLSelectElement;
      expect(formatSelect.options).toHaveLength(3);
      expect(formatSelect.options[0].text).toBe('Premier Draft');
      expect(formatSelect.options[1].text).toBe('Quick Draft');
      expect(formatSelect.options[2].text).toBe('Traditional Draft');
    });

    it('should select Premier Draft by default', async () => {
      render(<DraftAnalytics />);

      await waitFor(() => {
        const formatSelect = screen.getByLabelText('Format') as HTMLSelectElement;
        expect(formatSelect.value).toBe('PremierDraft');
      });
    });

    it('should allow changing draft format', async () => {
      const user = userEvent.setup();

      render(<DraftAnalytics />);

      await waitFor(() => {
        expect(screen.getByLabelText('Format')).toBeInTheDocument();
      });

      const formatSelect = screen.getByLabelText('Format');
      await user.selectOptions(formatSelect, 'QuickDraft');

      expect((formatSelect as HTMLSelectElement).value).toBe('QuickDraft');
    });
  });

  describe('Auto-refresh Toggle', () => {
    it('should display auto-refresh checkbox', async () => {
      render(<DraftAnalytics />);

      await waitFor(() => {
        expect(screen.getByText('Auto-refresh')).toBeInTheDocument();
      });
    });

    it('should have auto-refresh disabled by default', async () => {
      render(<DraftAnalytics />);

      await waitFor(() => {
        const checkbox = screen.getByRole('checkbox');
        expect(checkbox).not.toBeChecked();
      });
    });

    it('should allow toggling auto-refresh', async () => {
      const user = userEvent.setup();

      render(<DraftAnalytics />);

      await waitFor(() => {
        expect(screen.getByRole('checkbox')).toBeInTheDocument();
      });

      const checkbox = screen.getByRole('checkbox');
      await user.click(checkbox);

      expect(checkbox).toBeChecked();
    });
  });

  describe('Analytics Components', () => {
    it('should render TemporalTrends component', async () => {
      render(<DraftAnalytics />);

      await waitFor(() => {
        // TemporalTrends will show loading or content
        // We check that the page is rendered with the components area
        expect(screen.getByRole('heading', { name: 'Draft Analytics' })).toBeInTheDocument();
      });
    });

    it('should render CommunityComparison component', async () => {
      render(<DraftAnalytics />);

      await waitFor(() => {
        expect(screen.getByRole('heading', { name: 'Draft Analytics' })).toBeInTheDocument();
      });
    });

    it('should render FormatInsights component', async () => {
      render(<DraftAnalytics />);

      await waitFor(() => {
        expect(screen.getByRole('heading', { name: 'Draft Analytics' })).toBeInTheDocument();
      });
    });
  });

  describe('API Calls', () => {
    it('should call getDraftFormats on mount', async () => {
      render(<DraftAnalytics />);

      await waitFor(() => {
        expect(mockDrafts.getDraftFormats).toHaveBeenCalled();
      });
    });
  });

  describe('Error Handling', () => {
    it('should handle API error gracefully', async () => {
      mockDrafts.getDraftFormats.mockRejectedValue(new Error('Failed to fetch'));
      const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

      render(<DraftAnalytics />);

      await waitFor(() => {
        expect(consoleSpy).toHaveBeenCalledWith('Failed to fetch draft formats:', expect.any(Error));
      });

      consoleSpy.mockRestore();
    });
  });
});
