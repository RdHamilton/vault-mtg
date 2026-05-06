import './EmptyState.css';

export type EmptyStateVariant = 'no-data' | 'coming-soon';

export interface EmptyStateProps {
  /** Optional icon character or emoji shown above the heading */
  icon?: string;
  /** Primary heading text */
  heading: string;
  /** Supporting body text */
  subtext: string;
  /** CTA button label — only rendered for 'no-data' variant when ctaHref also provided */
  ctaLabel?: string;
  /** CTA href — only rendered for 'no-data' variant when ctaLabel also provided */
  ctaHref?: string;
  /** 'no-data'     — feature works, user has no records (CTA optional)
   *  'coming-soon' — feature not yet implemented, no CTA rendered */
  variant?: EmptyStateVariant;
}

const EmptyState = ({
  icon,
  heading,
  subtext,
  ctaLabel,
  ctaHref,
  variant = 'no-data',
}: EmptyStateProps) => {
  const showCta = variant === 'no-data' && ctaLabel && ctaHref;

  return (
    <div className={`empty-state empty-state--${variant}`} data-testid="empty-state">
      {icon && <div className="empty-state-icon">{icon}</div>}
      <h2 className="empty-state-heading">{heading}</h2>
      <p className="empty-state-subtext">{subtext}</p>
      {showCta && (
        <a href={ctaHref} className="empty-state-cta">
          {ctaLabel}
        </a>
      )}
    </div>
  );
};

export default EmptyState;
