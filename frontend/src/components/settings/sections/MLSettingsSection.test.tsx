import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MLSettingsSection } from './MLSettingsSection';

vi.mock('../../../services/api/mlSuggestions', () => ({
  clearLearnedData: vi.fn(),
}));

import { clearLearnedData } from '../../../services/api/mlSuggestions';

const defaultProps = {
  mlEnabled: true,
  onMLEnabledChange: vi.fn(),
  metaGoldfishEnabled: true,
  onMetaGoldfishEnabledChange: vi.fn(),
  metaTop8Enabled: true,
  onMetaTop8EnabledChange: vi.fn(),
  metaWeight: 0.3,
  onMetaWeightChange: vi.fn(),
  personalWeight: 0.2,
  onPersonalWeightChange: vi.fn(),
  // ML Suggestion Preferences
  suggestionFrequency: 'medium',
  onSuggestionFrequencyChange: vi.fn(),
  minimumConfidence: 50,
  onMinimumConfidenceChange: vi.fn(),
  showCardAdditions: true,
  onShowCardAdditionsChange: vi.fn(),
  showCardRemovals: true,
  onShowCardRemovalsChange: vi.fn(),
  showArchetypeChanges: true,
  onShowArchetypeChangesChange: vi.fn(),
  learnFromMatches: true,
  onLearnFromMatchesChange: vi.fn(),
  learnFromDeckChanges: true,
  onLearnFromDeckChangesChange: vi.fn(),
  retentionDays: 90,
  onRetentionDaysChange: vi.fn(),
  maxSuggestionsPerView: 5,
  onMaxSuggestionsPerViewChange: vi.fn(),
};

describe('MLSettingsSection', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('rendering', () => {
    it('renders the section title', () => {
      render(<MLSettingsSection {...defaultProps} />);
      expect(screen.getByText('ML Recommendations')).toBeInTheDocument();
    });

    it('renders the ML enabled toggle', () => {
      render(<MLSettingsSection {...defaultProps} />);
      expect(screen.getByText('Enable ML Recommendations')).toBeInTheDocument();
    });

    it('shows meta data sources section when ML is enabled', () => {
      render(<MLSettingsSection {...defaultProps} mlEnabled={true} />);
      expect(screen.getByText('Meta Data Sources')).toBeInTheDocument();
      expect(screen.getByText('MTGGoldfish Metagame Data')).toBeInTheDocument();
      expect(screen.getByText('MTGTop8 Tournament Results')).toBeInTheDocument();
    });

    it('shows recommendation weights section when ML is enabled', () => {
      render(<MLSettingsSection {...defaultProps} mlEnabled={true} />);
      expect(screen.getByText('Recommendation Weights')).toBeInTheDocument();
      expect(screen.getByText('Meta Weight')).toBeInTheDocument();
      expect(screen.getByText('Personal History Weight')).toBeInTheDocument();
    });

    it('hides all subsections when ML is disabled', () => {
      render(<MLSettingsSection {...defaultProps} mlEnabled={false} />);
      expect(screen.queryByText('Meta Data Sources')).not.toBeInTheDocument();
      expect(screen.queryByText('Recommendation Weights')).not.toBeInTheDocument();
      expect(screen.queryByText('Suggestion Preferences')).not.toBeInTheDocument();
    });

    it('does not render any Ollama / LLM UI', () => {
      render(<MLSettingsSection {...defaultProps} />);
      expect(screen.queryByText(/Ollama/i)).not.toBeInTheDocument();
      expect(screen.queryByText(/LLM/i)).not.toBeInTheDocument();
    });
  });

  describe('ml enabled toggle', () => {
    it('calls onMLEnabledChange when toggled', () => {
      const onMLEnabledChange = vi.fn();
      render(<MLSettingsSection {...defaultProps} onMLEnabledChange={onMLEnabledChange} />);

      const toggle = screen.getAllByRole('checkbox')[0];
      fireEvent.click(toggle);

      expect(onMLEnabledChange).toHaveBeenCalled();
    });
  });

  describe('meta data sources', () => {
    it('renders MTGGoldfish toggle with correct state', () => {
      render(<MLSettingsSection {...defaultProps} metaGoldfishEnabled={true} />);
      const toggles = screen.getAllByRole('checkbox');
      // Second toggle: MTGGoldfish (first is mlEnabled)
      expect(toggles[1]).toBeChecked();
    });

    it('calls onMetaGoldfishEnabledChange when toggled', () => {
      const onMetaGoldfishEnabledChange = vi.fn();
      render(
        <MLSettingsSection
          {...defaultProps}
          onMetaGoldfishEnabledChange={onMetaGoldfishEnabledChange}
        />
      );

      const toggles = screen.getAllByRole('checkbox');
      fireEvent.click(toggles[1]);

      expect(onMetaGoldfishEnabledChange).toHaveBeenCalled();
    });
  });

  describe('weights', () => {
    it('displays meta weight value', () => {
      render(<MLSettingsSection {...defaultProps} metaWeight={0.4} />);
      const values = screen.getAllByText('0.4');
      expect(values.length).toBeGreaterThan(0);
    });

    it('calls onMetaWeightChange when slider changes', () => {
      const onMetaWeightChange = vi.fn();
      render(<MLSettingsSection {...defaultProps} onMetaWeightChange={onMetaWeightChange} />);

      const sliders = screen.getAllByRole('slider');
      fireEvent.change(sliders[0], { target: { value: '0.5' } });

      expect(onMetaWeightChange).toHaveBeenCalledWith(0.5);
    });
  });

  describe('suggestion preferences', () => {
    it('renders suggestion frequency select', () => {
      render(<MLSettingsSection {...defaultProps} />);
      expect(screen.getByText('Suggestion Frequency')).toBeInTheDocument();
    });

    it('renders minimum confidence slider with current value', () => {
      render(<MLSettingsSection {...defaultProps} minimumConfidence={75} />);
      expect(screen.getByText('75%')).toBeInTheDocument();
    });

    it('renders max suggestions per view', () => {
      render(<MLSettingsSection {...defaultProps} />);
      expect(screen.getByText('Max Suggestions Per View')).toBeInTheDocument();
    });
  });

  describe('suggestion types', () => {
    it('renders all three suggestion type toggles', () => {
      render(<MLSettingsSection {...defaultProps} />);
      expect(screen.getByText('Card Additions')).toBeInTheDocument();
      expect(screen.getByText('Card Removals')).toBeInTheDocument();
      expect(screen.getByText('Archetype Changes')).toBeInTheDocument();
    });
  });

  describe('clear learned data', () => {
    it('renders clear all data button', () => {
      render(<MLSettingsSection {...defaultProps} />);
      expect(screen.getByRole('button', { name: 'Clear All Data' })).toBeInTheDocument();
    });

    it('skips the API call when the user cancels the confirm dialog', () => {
      const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);
      render(<MLSettingsSection {...defaultProps} />);

      fireEvent.click(screen.getByRole('button', { name: 'Clear All Data' }));

      expect(clearLearnedData).not.toHaveBeenCalled();
      confirmSpy.mockRestore();
    });

    it('calls clearLearnedData and shows success message on confirm', async () => {
      const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
      vi.mocked(clearLearnedData).mockResolvedValueOnce({ status: 'ok', message: 'done' });

      render(<MLSettingsSection {...defaultProps} />);
      fireEvent.click(screen.getByRole('button', { name: 'Clear All Data' }));

      await waitFor(() => {
        expect(clearLearnedData).toHaveBeenCalled();
      });
      await waitFor(() => {
        expect(screen.getByText(/has been cleared successfully/i)).toBeInTheDocument();
      });
      confirmSpy.mockRestore();
    });

    it('shows error message when clearLearnedData fails', async () => {
      const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
      vi.mocked(clearLearnedData).mockRejectedValueOnce(new Error('boom'));

      render(<MLSettingsSection {...defaultProps} />);
      fireEvent.click(screen.getByRole('button', { name: 'Clear All Data' }));

      await waitFor(() => {
        expect(screen.getByText(/Failed to clear data: boom/i)).toBeInTheDocument();
      });
      confirmSpy.mockRestore();
    });
  });

  describe('about info', () => {
    it('renders the about ML recommendations block', () => {
      render(<MLSettingsSection {...defaultProps} mlEnabled={true} />);
      expect(screen.getByText(/About ML Recommendations:/i)).toBeInTheDocument();
    });
  });
});
