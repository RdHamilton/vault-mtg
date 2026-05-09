import { useCallback } from 'react';
import { useUser } from '@clerk/react';
import * as Sentry from '@sentry/react';
import './ReportBugButton.css';

/**
 * ReportBugButton — floating "Report a bug" trigger for authenticated users.
 *
 * Opens the Sentry User Feedback dialog, pre-populated with the signed-in
 * user's name and email from Clerk so CS gets full identity context alongside
 * the Sentry session / error trace.
 *
 * Only rendered when the user is signed in (enforced by the parent Layout).
 */
const ReportBugButton = () => {
  const { user, isSignedIn } = useUser();

  const handleClick = useCallback(async () => {
    if (!isSignedIn || !user) return;

    const primaryEmail = user.emailAddresses?.[0]?.emailAddress ?? '';
    const name = user.fullName ?? [user.firstName, user.lastName].filter(Boolean).join(' ');

    // getFeedback() returns the feedbackIntegration instance (added in main.tsx).
    // If Sentry is not initialised (dev without VITE_SENTRY_DSN), getFeedback()
    // returns undefined — guard and bail silently.
    const feedback = Sentry.getFeedback();
    if (!feedback) return;

    // Sentry v10 feedback API: createForm() returns a Promise<FeedbackDialog>.
    // The dialog must be appended to the DOM and then opened. We pre-fill the
    // form via the integration's `useSentryUser` option pattern by passing
    // overrides — see sentry.io/docs/platforms/javascript/user-feedback.
    const dialog = await feedback.createForm({
      useSentryUser: {
        name: name || '',
        email: primaryEmail || '',
      },
    });
    dialog.appendToDom();
    dialog.open();
  }, [isSignedIn, user]);

  if (!isSignedIn) return null;

  return (
    <button
      className="report-bug-btn"
      onClick={handleClick}
      aria-label="Report a bug"
      data-testid="report-bug-button"
      type="button"
    >
      Report a bug
    </button>
  );
};

export default ReportBugButton;
