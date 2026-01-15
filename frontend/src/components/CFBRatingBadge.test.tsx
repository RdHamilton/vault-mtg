import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { CFBRatingBadge } from './CFBRatingBadge';
import { getCFBGradeColor, ratingToGrade } from '@/services/api/cards';

describe('CFBRatingBadge', () => {
  it('renders the grade text for a rating', () => {
    render(<CFBRatingBadge rating={4.5} />); // 4.5 = 'A'
    expect(screen.getByText('A')).toBeInTheDocument();
  });

  it('renders the CFB label by default', () => {
    render(<CFBRatingBadge rating={3.5} />); // 3.5 = 'B+'
    expect(screen.getByText('CFB')).toBeInTheDocument();
    expect(screen.getByText('B+')).toBeInTheDocument();
  });

  it('hides the CFB label when showLabel is false', () => {
    render(<CFBRatingBadge rating={4.0} showLabel={false} />); // 4.0 = 'A-'
    expect(screen.queryByText('CFB')).not.toBeInTheDocument();
    expect(screen.getByText('A-')).toBeInTheDocument();
  });

  it('applies the correct background color for each rating range', () => {
    // Test representative ratings from each grade range
    const ratingTests = [
      { rating: 5.0, expectedGrade: 'A+' },
      { rating: 4.5, expectedGrade: 'A' },
      { rating: 4.0, expectedGrade: 'A-' },
      { rating: 3.5, expectedGrade: 'B+' },
      { rating: 3.0, expectedGrade: 'B' },
      { rating: 2.5, expectedGrade: 'B-' },
      { rating: 2.0, expectedGrade: 'C+' },
      { rating: 1.5, expectedGrade: 'C' },
      { rating: 1.0, expectedGrade: 'C-' },
      { rating: 0.5, expectedGrade: 'D' },
      { rating: 0.0, expectedGrade: 'F' },
    ];

    ratingTests.forEach(({ rating, expectedGrade }) => {
      const { unmount } = render(<CFBRatingBadge rating={rating} />);
      const badge = screen.getByTestId('cfb-rating-badge');
      expect(badge).toHaveStyle({ backgroundColor: getCFBGradeColor(expectedGrade as ReturnType<typeof ratingToGrade>) });
      unmount();
    });
  });

  it('shows default tooltip with rating and grade', () => {
    render(<CFBRatingBadge rating={3.0} />); // 3.0 = 'B'
    const badge = screen.getByTestId('cfb-rating-badge');
    expect(badge).toHaveAttribute('title', 'CFB Rating: 3.0 (B)');
  });

  it('shows custom commentary as tooltip', () => {
    render(<CFBRatingBadge rating={5.0} commentary="Best card in the set!" />);
    const badge = screen.getByTestId('cfb-rating-badge');
    expect(badge).toHaveAttribute('title', 'Best card in the set!');
  });

  it('applies small size class', () => {
    render(<CFBRatingBadge rating={1.5} size="small" />);
    const badge = screen.getByTestId('cfb-rating-badge');
    expect(badge).toHaveClass('cfb-rating-badge--small');
  });

  it('applies medium size class by default', () => {
    render(<CFBRatingBadge rating={1.5} />);
    const badge = screen.getByTestId('cfb-rating-badge');
    expect(badge).toHaveClass('cfb-rating-badge--medium');
  });

  it('applies large size class', () => {
    render(<CFBRatingBadge rating={1.5} size="large" />);
    const badge = screen.getByTestId('cfb-rating-badge');
    expect(badge).toHaveClass('cfb-rating-badge--large');
  });

  it('shows numerical rating when showNumeric is true', () => {
    render(<CFBRatingBadge rating={3.75} showNumeric={true} />);
    expect(screen.getByText('3.8')).toBeInTheDocument();
  });

  it('shows letter grade by default', () => {
    render(<CFBRatingBadge rating={3.75} />); // 3.75 = 'A-'
    expect(screen.getByText('A-')).toBeInTheDocument();
  });
});

describe('ratingToGrade', () => {
  it('converts 5.0 to A+', () => {
    expect(ratingToGrade(5.0)).toBe('A+');
  });

  it('converts 4.5 to A', () => {
    expect(ratingToGrade(4.5)).toBe('A');
  });

  it('converts 4.0 to A-', () => {
    expect(ratingToGrade(4.0)).toBe('A-');
  });

  it('converts 3.5 to B+', () => {
    expect(ratingToGrade(3.5)).toBe('B+');
  });

  it('converts 3.0 to B', () => {
    expect(ratingToGrade(3.0)).toBe('B');
  });

  it('converts 2.5 to B-', () => {
    expect(ratingToGrade(2.5)).toBe('B-');
  });

  it('converts 2.0 to C+', () => {
    expect(ratingToGrade(2.0)).toBe('C+');
  });

  it('converts 1.5 to C', () => {
    expect(ratingToGrade(1.5)).toBe('C');
  });

  it('converts 1.0 to C-', () => {
    expect(ratingToGrade(1.0)).toBe('C-');
  });

  it('converts 0.5 to D', () => {
    expect(ratingToGrade(0.5)).toBe('D');
  });

  it('converts 0.0 to F', () => {
    expect(ratingToGrade(0.0)).toBe('F');
  });

  it('handles edge cases near boundaries', () => {
    expect(ratingToGrade(4.74)).toBe('A');
    expect(ratingToGrade(4.75)).toBe('A+');
    expect(ratingToGrade(2.74)).toBe('B-');
    expect(ratingToGrade(2.75)).toBe('B');
  });
});

describe('getCFBGradeColor', () => {
  it('returns gold for A+', () => {
    expect(getCFBGradeColor('A+')).toBe('#ffd700');
  });

  it('returns silver for A', () => {
    expect(getCFBGradeColor('A')).toBe('#c0c0c0');
  });

  it('returns bronze for B+', () => {
    expect(getCFBGradeColor('B+')).toBe('#cd7f32');
  });

  it('returns blue for C+', () => {
    expect(getCFBGradeColor('C+')).toBe('#4a9eff');
  });

  it('returns gray for D', () => {
    expect(getCFBGradeColor('D')).toBe('#888888');
  });

  it('returns red for F', () => {
    expect(getCFBGradeColor('F')).toBe('#ff4444');
  });
});
