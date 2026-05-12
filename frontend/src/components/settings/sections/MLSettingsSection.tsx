import { useState } from 'react';
import LoadingButton from '../../LoadingButton';
import { SettingItem, SettingToggle, SettingSelect } from '../';
import { clearLearnedData } from '../../../services/api/mlSuggestions';

export interface MLSettingsSectionProps {
  mlEnabled: boolean;
  onMLEnabledChange: (enabled: boolean) => void;
  metaGoldfishEnabled: boolean;
  onMetaGoldfishEnabledChange: (enabled: boolean) => void;
  metaTop8Enabled: boolean;
  onMetaTop8EnabledChange: (enabled: boolean) => void;
  metaWeight: number;
  onMetaWeightChange: (weight: number) => void;
  personalWeight: number;
  onPersonalWeightChange: (weight: number) => void;
  // ML Suggestion Preferences
  suggestionFrequency: string;
  onSuggestionFrequencyChange: (frequency: string) => void;
  minimumConfidence: number;
  onMinimumConfidenceChange: (confidence: number) => void;
  showCardAdditions: boolean;
  onShowCardAdditionsChange: (show: boolean) => void;
  showCardRemovals: boolean;
  onShowCardRemovalsChange: (show: boolean) => void;
  showArchetypeChanges: boolean;
  onShowArchetypeChangesChange: (show: boolean) => void;
  learnFromMatches: boolean;
  onLearnFromMatchesChange: (learn: boolean) => void;
  learnFromDeckChanges: boolean;
  onLearnFromDeckChangesChange: (learn: boolean) => void;
  retentionDays: number;
  onRetentionDaysChange: (days: number) => void;
  maxSuggestionsPerView: number;
  onMaxSuggestionsPerViewChange: (max: number) => void;
}

export function MLSettingsSection(props: MLSettingsSectionProps) {
  const {
    mlEnabled,
    onMLEnabledChange,
    metaGoldfishEnabled,
    onMetaGoldfishEnabledChange,
    metaTop8Enabled,
    onMetaTop8EnabledChange,
    metaWeight,
    onMetaWeightChange,
    personalWeight,
    onPersonalWeightChange,
    // ML Suggestion Preferences
    suggestionFrequency,
    onSuggestionFrequencyChange,
    minimumConfidence,
    onMinimumConfidenceChange,
    showCardAdditions,
    onShowCardAdditionsChange,
    showCardRemovals,
    onShowCardRemovalsChange,
    showArchetypeChanges,
    onShowArchetypeChangesChange,
    learnFromMatches,
    onLearnFromMatchesChange,
    learnFromDeckChanges,
    onLearnFromDeckChangesChange,
    retentionDays,
    onRetentionDaysChange,
    maxSuggestionsPerView,
    onMaxSuggestionsPerViewChange,
  } = props;

  const [isClearingData, setIsClearingData] = useState(false);
  const [clearDataMessage, setClearDataMessage] = useState<string | null>(null);

  const handleClearLearnedData = async () => {
    if (!window.confirm('Are you sure you want to clear all learned ML data? This cannot be undone.')) {
      return;
    }
    setIsClearingData(true);
    setClearDataMessage(null);
    try {
      await clearLearnedData();
      setClearDataMessage('All learned data has been cleared successfully.');
    } catch (error) {
      setClearDataMessage(`Failed to clear data: ${error instanceof Error ? error.message : 'Unknown error'}`);
    } finally {
      setIsClearingData(false);
    }
  };

  return (
    <div className="settings-section">
      <h2 className="section-title">ML Recommendations</h2>
      <div className="setting-description settings-section-description">
        Configure machine learning-powered card recommendations.
        These features enhance deck building with personalized suggestions based on your play style
        and current metagame data.
      </div>

      {/* ML Recommendations Toggle */}
      <SettingToggle
        label="Enable ML Recommendations"
        description="Use machine learning to provide personalized card recommendations based on your play history and deck composition"
        checked={mlEnabled}
        onChange={onMLEnabledChange}
      />

      {mlEnabled && (
        <>
          {/* Meta Data Sources Section */}
          <div className="settings-subsection">
            <h3 className="subsection-title">Meta Data Sources</h3>
            <div className="setting-description">
              Configure which external data sources to use for metagame-aware recommendations.
            </div>

            <SettingToggle
              label="MTGGoldfish Metagame Data"
              description="Include deck archetypes and meta shares from MTGGoldfish for constructed formats"
              checked={metaGoldfishEnabled}
              onChange={onMetaGoldfishEnabledChange}
            />

            <SettingToggle
              label="MTGTop8 Tournament Results"
              description="Include tournament performance data and winning decklists from MTGTop8"
              checked={metaTop8Enabled}
              onChange={onMetaTop8EnabledChange}
            />
          </div>

          {/* Weight Configuration */}
          <div className="settings-subsection">
            <h3 className="subsection-title">Recommendation Weights</h3>
            <div className="setting-description">
              Adjust how different factors influence card recommendations. Higher weights mean more influence.
            </div>

            <SettingItem
              label="Meta Weight"
              description="How much metagame data influences recommendations (0-1)"
            >
              <input
                type="range"
                min="0"
                max="1"
                step="0.1"
                value={metaWeight}
                onChange={(e) => onMetaWeightChange(parseFloat(e.target.value))}
                className="slider-input"
              />
              <span className="slider-value">{metaWeight.toFixed(1)}</span>
            </SettingItem>

            <SettingItem
              label="Personal History Weight"
              description="How much your personal play history influences recommendations (0-1)"
            >
              <input
                type="range"
                min="0"
                max="1"
                step="0.1"
                value={personalWeight}
                onChange={(e) => onPersonalWeightChange(parseFloat(e.target.value))}
                className="slider-input"
              />
              <span className="slider-value">{personalWeight.toFixed(1)}</span>
            </SettingItem>
          </div>

          {/* Suggestion Preferences Section */}
          <div className="settings-subsection">
            <h3 className="subsection-title">Suggestion Preferences</h3>
            <div className="setting-description">
              Customize how ML suggestions are generated and displayed.
            </div>

            <SettingSelect
              label="Suggestion Frequency"
              description="How often suggestions are generated when viewing decks"
              value={suggestionFrequency}
              onChange={onSuggestionFrequencyChange}
              options={[
                { value: 'low', label: 'Low (less aggressive)' },
                { value: 'medium', label: 'Medium (balanced)' },
                { value: 'high', label: 'High (more suggestions)' },
              ]}
            />

            <SettingItem
              label="Minimum Confidence"
              description="Only show suggestions with confidence above this threshold"
            >
              <input
                type="range"
                min="0"
                max="100"
                step="5"
                value={minimumConfidence}
                onChange={(e) => onMinimumConfidenceChange(parseInt(e.target.value))}
                className="slider-input"
              />
              <span className="slider-value">{minimumConfidence}%</span>
            </SettingItem>

            <SettingSelect
              label="Max Suggestions Per View"
              description="Maximum number of suggestions to show at once"
              value={String(maxSuggestionsPerView)}
              onChange={(v) => onMaxSuggestionsPerViewChange(parseInt(v))}
              options={[
                { value: '3', label: '3 suggestions' },
                { value: '5', label: '5 suggestions' },
                { value: '10', label: '10 suggestions' },
              ]}
            />
          </div>

          {/* Suggestion Types Section */}
          <div className="settings-subsection">
            <h3 className="subsection-title">Suggestion Types</h3>
            <div className="setting-description">
              Choose which types of suggestions to show.
            </div>

            <SettingToggle
              label="Card Additions"
              description="Suggest cards to add to your deck"
              checked={showCardAdditions}
              onChange={onShowCardAdditionsChange}
            />

            <SettingToggle
              label="Card Removals"
              description="Suggest underperforming cards to remove"
              checked={showCardRemovals}
              onChange={onShowCardRemovalsChange}
            />

            <SettingToggle
              label="Archetype Changes"
              description="Suggest strategic shifts in deck direction"
              checked={showArchetypeChanges}
              onChange={onShowArchetypeChangesChange}
            />
          </div>

          {/* Learning Options Section */}
          <div className="settings-subsection">
            <h3 className="subsection-title">Learning Options</h3>
            <div className="setting-description">
              Control how the ML engine learns from your activity.
            </div>

            <SettingToggle
              label="Learn from Match Results"
              description="Improve suggestions based on your win/loss outcomes"
              checked={learnFromMatches}
              onChange={onLearnFromMatchesChange}
            />

            <SettingToggle
              label="Learn from Deck Changes"
              description="Improve suggestions based on card swaps you make"
              checked={learnFromDeckChanges}
              onChange={onLearnFromDeckChangesChange}
            />

            <SettingSelect
              label="Data Retention"
              description="How long to keep learned data before clearing old entries"
              value={String(retentionDays)}
              onChange={(v) => onRetentionDaysChange(parseInt(v))}
              options={[
                { value: '30', label: '30 days' },
                { value: '90', label: '90 days' },
                { value: '180', label: '6 months' },
                { value: '365', label: '1 year' },
                { value: '-1', label: 'Forever' },
              ]}
            />

            <SettingItem
              label="Clear Learned Data"
              description="Remove all ML learned data (synergies, patterns, suggestions)"
            >
              <LoadingButton
                loading={isClearingData}
                loadingText="Clearing..."
                onClick={handleClearLearnedData}
                variant="danger"
              >
                Clear All Data
              </LoadingButton>
            </SettingItem>

            {clearDataMessage && (
              <div className={`setting-hint ${clearDataMessage.includes('Failed') ? 'settings-error-box' : 'settings-success-box'}`}>
                {clearDataMessage}
              </div>
            )}
          </div>

          <div className="setting-hint settings-info-box">
            <strong>About ML Recommendations:</strong>
            <ul className="info-list">
              <li>Recommendations are based on your personal play history, deck composition, and current metagame</li>
              <li>The ML model learns from your match results to improve suggestions over time</li>
              <li>All processing happens on the BFF — your account-scoped data never goes to third-party services</li>
            </ul>
          </div>
        </>
      )}
    </div>
  );
}
