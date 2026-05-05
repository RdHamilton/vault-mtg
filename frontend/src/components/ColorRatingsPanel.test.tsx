import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import ColorRatingsPanel from './ColorRatingsPanel';
import { BffColorRating } from '@/services/api/bffDraftRatings';

const mockRatings: BffColorRating[] = [
  { color_combination: 'WU', win_rate: 0.579, games_played: 4200 },
  { color_combination: 'BR', win_rate: 0.541, games_played: 3800 },
  { color_combination: 'G',  win_rate: 0.498, games_played: 1500 },
  { color_combination: 'UB', win_rate: 0.471, games_played: 2100 },
];

describe('ColorRatingsPanel', () => {
  it('renders the panel with a title', () => {
    render(<ColorRatingsPanel colorRatings={mockRatings} />);
    expect(screen.getByTestId('color-ratings-panel')).toBeInTheDocument();
    expect(screen.getByText('Color Win Rates')).toBeInTheDocument();
  });

  it('renders a row for each color combination with win rates', () => {
    render(<ColorRatingsPanel colorRatings={mockRatings} />);
    expect(screen.getByTestId('color-rating-WU')).toBeInTheDocument();
    expect(screen.getByTestId('color-rating-BR')).toBeInTheDocument();
    expect(screen.getByTestId('color-rating-G')).toBeInTheDocument();
    expect(screen.getByTestId('color-rating-UB')).toBeInTheDocument();
  });

  it('displays the correct win rate percentages', () => {
    render(<ColorRatingsPanel colorRatings={mockRatings} />);
    expect(screen.getByText('57.9%')).toBeInTheDocument();
    expect(screen.getByText('54.1%')).toBeInTheDocument();
    expect(screen.getByText('49.8%')).toBeInTheDocument();
    expect(screen.getByText('47.1%')).toBeInTheDocument();
  });

  it('shows human-readable color names', () => {
    render(<ColorRatingsPanel colorRatings={mockRatings} />);
    expect(screen.getByText('Azorius')).toBeInTheDocument();
    expect(screen.getByText('Rakdos')).toBeInTheDocument();
    expect(screen.getByText('Green')).toBeInTheDocument();
    expect(screen.getByText('Dimir')).toBeInTheDocument();
  });

  it('displays games played when provided', () => {
    render(<ColorRatingsPanel colorRatings={mockRatings} />);
    expect(screen.getByText('(4,200 games)')).toBeInTheDocument();
  });

  it('sorts ratings by win rate descending', () => {
    render(<ColorRatingsPanel colorRatings={mockRatings} />);
    const rows = screen.getAllByTestId(/^color-rating-/);
    // First row should be the highest win-rate combination (WU = 57.9%)
    expect(rows[0]).toHaveAttribute('data-testid', 'color-rating-WU');
    // Last row should be the lowest (UB = 47.1%)
    expect(rows[rows.length - 1]).toHaveAttribute('data-testid', 'color-rating-UB');
  });

  it('returns null when colorRatings is empty', () => {
    const { container } = render(<ColorRatingsPanel colorRatings={[]} />);
    expect(container.firstChild).toBeNull();
  });

  it('returns null when colorRatings is all entries without win_rate', () => {
    const noRates: BffColorRating[] = [{ color_combination: 'W' }];
    const { container } = render(<ColorRatingsPanel colorRatings={noRates} />);
    expect(container.firstChild).toBeNull();
  });

  it('handles unknown color combinations gracefully', () => {
    const weird: BffColorRating[] = [{ color_combination: 'XYZ', win_rate: 0.52 }];
    render(<ColorRatingsPanel colorRatings={weird} />);
    expect(screen.getByTestId('color-rating-XYZ')).toBeInTheDocument();
    expect(screen.getByText('52.0%')).toBeInTheDocument();
  });

  it('applies wr-excellent class to win rates >= 57%', () => {
    const high: BffColorRating[] = [{ color_combination: 'WU', win_rate: 0.57 }];
    render(<ColorRatingsPanel colorRatings={high} />);
    const row = screen.getByTestId('color-rating-WU');
    expect(row).toHaveClass('wr-excellent');
  });

  it('applies wr-below class to win rates below 50%', () => {
    const low: BffColorRating[] = [{ color_combination: 'UB', win_rate: 0.47 }];
    render(<ColorRatingsPanel colorRatings={low} />);
    const row = screen.getByTestId('color-rating-UB');
    expect(row).toHaveClass('wr-below');
  });

  it('applies wr-good class to win rates 54–57%', () => {
    const good: BffColorRating[] = [{ color_combination: 'BR', win_rate: 0.541 }];
    render(<ColorRatingsPanel colorRatings={good} />);
    const row = screen.getByTestId('color-rating-BR');
    expect(row).toHaveClass('wr-good');
  });

  it('applies wr-average class to win rates 50–54%', () => {
    const avg: BffColorRating[] = [{ color_combination: 'WG', win_rate: 0.52 }];
    render(<ColorRatingsPanel colorRatings={avg} />);
    const row = screen.getByTestId('color-rating-WG');
    expect(row).toHaveClass('wr-average');
  });
});
