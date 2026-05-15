import React, { useState, useEffect } from 'react';
import { apiAdapter } from '@/services/adapter';
import TemporalTrends from '@/components/TemporalTrends';
import CommunityComparison from '@/components/CommunityComparison';
import FormatInsights from '@/components/FormatInsights';
import './DraftAnalytics.css';

const DraftAnalytics: React.FC = () => {
  const [availableSets, setAvailableSets] = useState<string[]>([]);
  const [selectedSet, setSelectedSet] = useState<string>('');
  const [draftFormat, setDraftFormat] = useState<string>('PremierDraft');
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    async function fetchSets() {
      try {
        const formats = await apiAdapter.drafts.getDraftFormats();
        setAvailableSets(formats);
        if (formats.length > 0) {
          setSelectedSet((currentSet) => currentSet || formats[0]);
        }
      } catch (err) {
        console.error('Failed to fetch draft formats:', err);
      } finally {
        setLoading(false);
      }
    }
    fetchSets();
  }, []);

  if (loading) {
    return (
      <div className="draft-analytics draft-analytics--loading">
        <div className="draft-analytics__spinner" />
        <span>Loading draft analytics...</span>
      </div>
    );
  }

  if (availableSets.length === 0) {
    return (
      <div className="draft-analytics draft-analytics--empty">
        <h2>No Draft Data Available</h2>
        <p>Complete some drafts to see your analytics and performance trends.</p>
      </div>
    );
  }

  return (
    <div className="draft-analytics">
      <div className="draft-analytics__header">
        <h1>Draft Analytics</h1>
        <div className="draft-analytics__filters">
          <div className="draft-analytics__filter">
            <label htmlFor="set-select">Set</label>
            <select
              id="set-select"
              value={selectedSet}
              onChange={(e) => setSelectedSet(e.target.value)}
              className="draft-analytics__select"
            >
              {availableSets.map((set) => (
                <option key={set} value={set}>
                  {set}
                </option>
              ))}
            </select>
          </div>
          <div className="draft-analytics__filter">
            <label htmlFor="format-select">Format</label>
            <select
              id="format-select"
              value={draftFormat}
              onChange={(e) => setDraftFormat(e.target.value)}
              className="draft-analytics__select"
            >
              <option value="PremierDraft">Premier Draft</option>
              <option value="QuickDraft">Quick Draft</option>
              <option value="TradDraft">Traditional Draft</option>
            </select>
          </div>
          <div className="draft-analytics__filter draft-analytics__filter--toggle">
            <label>
              <input
                type="checkbox"
                checked={autoRefresh}
                onChange={(e) => setAutoRefresh(e.target.checked)}
              />
              Auto-refresh
            </label>
          </div>
        </div>
      </div>

      <div className="draft-analytics__content">
        <div className="draft-analytics__section draft-analytics__section--full">
          <TemporalTrends
            setCode={selectedSet}
            periodType="week"
            numPeriods={12}
            showLearningCurve={true}
            autoRefresh={autoRefresh}
          />
        </div>

        <div className="draft-analytics__row">
          <div className="draft-analytics__section">
            <CommunityComparison
              setCode={selectedSet}
              draftFormat={draftFormat}
              autoRefresh={autoRefresh}
            />
          </div>
          <div className="draft-analytics__section">
            <FormatInsights
              setCode={selectedSet}
              draftFormat={draftFormat}
              autoRefresh={autoRefresh}
            />
          </div>
        </div>
      </div>
    </div>
  );
};

export default DraftAnalytics;
