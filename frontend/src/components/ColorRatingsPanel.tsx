import React from 'react';
import { BffColorRating } from '@/services/api/bffDraftRatings';
import './ColorRatingsPanel.css';

interface ColorRatingsPanelProps {
  colorRatings: BffColorRating[];
}

/** Maps single-letter color codes and pairs to display labels. */
function colorLabel(combination: string): string {
  const map: Record<string, string> = {
    W: 'White',
    U: 'Blue',
    B: 'Black',
    R: 'Red',
    G: 'Green',
    WU: 'Azorius',
    WB: 'Orzhov',
    WR: 'Boros',
    WG: 'Selesnya',
    UB: 'Dimir',
    UR: 'Izzet',
    UG: 'Simic',
    BR: 'Rakdos',
    BG: 'Golgari',
    RG: 'Gruul',
  };
  return map[combination.toUpperCase()] ?? combination;
}

/** Returns a color-coded CSS class based on the win rate. */
function winRateClass(winRate: number): string {
  if (winRate >= 57) return 'wr-excellent';
  if (winRate >= 54) return 'wr-good';
  if (winRate >= 50) return 'wr-average';
  return 'wr-below';
}

/** Single-letter color symbol emoji for MTG colors. */
function colorSymbol(combination: string): string {
  const symbols: Record<string, string> = {
    W: '☀',
    U: '💧',
    B: '💀',
    R: '🔥',
    G: '🌲',
  };
  if (combination.length === 1) return symbols[combination.toUpperCase()] ?? combination;
  return combination
    .toUpperCase()
    .split('')
    .map((c) => symbols[c] ?? c)
    .join('');
}

const ColorRatingsPanel: React.FC<ColorRatingsPanelProps> = ({ colorRatings }) => {
  if (!colorRatings || colorRatings.length === 0) {
    return null;
  }

  // Sort by win rate descending
  const sorted = [...colorRatings]
    .filter((cr) => cr.win_rate !== undefined && cr.win_rate !== null)
    .sort((a, b) => (b.win_rate ?? 0) - (a.win_rate ?? 0));

  if (sorted.length === 0) return null;

  return (
    <div className="color-ratings-panel" data-testid="color-ratings-panel">
      <h3 className="color-ratings-title">Color Win Rates</h3>
      <div className="color-ratings-list">
        {sorted.map((cr) => {
          const pct = ((cr.win_rate ?? 0) * 100).toFixed(1);
          const cls = winRateClass(Number(pct));
          return (
            <div
              key={cr.color_combination}
              className={`color-rating-row ${cls}`}
              data-testid={`color-rating-${cr.color_combination}`}
            >
              <span className="color-symbol" aria-hidden="true">
                {colorSymbol(cr.color_combination)}
              </span>
              <span className="color-name">{colorLabel(cr.color_combination)}</span>
              <span className={`win-rate ${cls}`}>{pct}%</span>
              {cr.games_played !== undefined && (
                <span className="games-played">({cr.games_played.toLocaleString('en-US')} games)</span>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
};

export default ColorRatingsPanel;
