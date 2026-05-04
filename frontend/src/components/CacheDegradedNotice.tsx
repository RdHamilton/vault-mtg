import React, { useState } from 'react';
import './CacheDegradedNotice.css';

interface CacheDegradedNoticeProps {
  /** When true the notice is rendered; when false nothing is rendered. */
  visible: boolean;
  /**
   * Number of hours since the cache was last refreshed from upstream.
   * When provided the notice appends "(N h ago)" to the message.
   */
  cacheAgeHours?: number;
}

/**
 * Subtle info-level banner shown when the BFF returns X-Cache-Degraded: true,
 * indicating that ratings data may be stale (live sync unavailable).
 *
 * Dismissible by the user for the current session.
 */
const CacheDegradedNotice: React.FC<CacheDegradedNoticeProps> = ({ visible, cacheAgeHours }) => {
  const [dismissed, setDismissed] = useState(false);

  if (!visible || dismissed) {
    return null;
  }

  const ageLabel =
    cacheAgeHours !== undefined
      ? ` (${Math.round(cacheAgeHours)} h ago)`
      : '';

  return (
    <div className="cache-degraded-notice" role="status" data-testid="cache-degraded-notice">
      <span className="cache-degraded-notice__icon" aria-hidden="true">&#x26A0;&#xFE0F;</span>
      <span className="cache-degraded-notice__message">
        Ratings data may be stale &mdash; live sync unavailable{ageLabel}
      </span>
      <button
        className="cache-degraded-notice__dismiss"
        aria-label="Dismiss stale data notice"
        onClick={() => setDismissed(true)}
      >
        &times;
      </button>
    </div>
  );
};

export default CacheDegradedNotice;
