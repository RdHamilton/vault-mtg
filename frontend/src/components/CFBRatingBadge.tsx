import { getCFBRatingColor, ratingToGrade } from '@/services/api/cards';
import './CFBRatingBadge.css';

export interface CFBRatingBadgeProps {
  /** The CFB limited rating (0.0-5.0 numerical scale) */
  rating: number;
  /** Optional commentary/tooltip text */
  commentary?: string;
  /** Size variant */
  size?: 'small' | 'medium' | 'large';
  /** Whether to show the "CFB" label */
  showLabel?: boolean;
  /** Whether to show numerical rating instead of letter grade */
  showNumeric?: boolean;
}

/**
 * Displays a ChannelFireball rating as a colored badge.
 * Accepts numerical rating (0-5 scale) and converts to letter grade for display.
 */
export function CFBRatingBadge({
  rating,
  commentary,
  size = 'medium',
  showLabel = true,
  showNumeric = false,
}: CFBRatingBadgeProps) {
  const color = getCFBRatingColor(rating);
  const grade = ratingToGrade(rating);
  const displayValue = showNumeric ? rating.toFixed(1) : grade;

  return (
    <span
      className={`cfb-rating-badge cfb-rating-badge--${size}`}
      style={{ backgroundColor: color }}
      title={commentary || `CFB Rating: ${rating.toFixed(1)} (${grade})`}
      data-testid="cfb-rating-badge"
    >
      {showLabel && <span className="cfb-rating-badge__label">CFB</span>}
      <span className="cfb-rating-badge__grade">{displayValue}</span>
    </span>
  );
}

export default CFBRatingBadge;
