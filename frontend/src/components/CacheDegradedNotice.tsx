import React, { useState } from 'react';
import './CacheDegradedNotice.css';

interface CacheDegradedNoticeProps {
  /** When true the notice is rendered; when false nothing is rendered. */
  visible: boolean;
}

/**
 * Subtle info-level banner shown when the BFF returns X-Cache-Degraded: true,
 * indicating that ratings data may be stale (live sync unavailable).
 *
 * Dismissible by the user for the current session.
 */
const CacheDegradedNotice: React.FC<CacheDegradedNoticeProps> = ({ visible }) => {
  const [dismissed, setDismissed] = useState(false);

  if (!visible || dismissed) {
    return null;
  }

  return (
    <div className="cache-degraded-notice" role="status" data-testid="cache-degraded-notice">
      <span className="cache-degraded-notice__icon" aria-hidden="true">&#x26A0;&#xFE0F;</span>
      <span className="cache-degraded-notice__message">
        Ratings data may be stale &mdash; live sync unavailable
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
